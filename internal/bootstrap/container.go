package bootstrap

import (
	"fmt"
	"log"
	"time"

	"github.com/Abraxas-365/freerouter/internal/access/accessmodule"
	"github.com/Abraxas-365/freerouter/internal/apikey/apikeymodule"
	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/gateway/gatewaymodule"
	"github.com/Abraxas-365/freerouter/internal/guardrail/guardrailmodule"
	"github.com/Abraxas-365/freerouter/internal/provider/providermodule"
	"github.com/Abraxas-365/freerouter/internal/providerkey/providerkeymodule"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/ratelimitmodule"
	"github.com/Abraxas-365/freerouter/internal/routingconfig/routingconfigmodule"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/usage/usagemodule"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhookmodule"
	"github.com/Abraxas-365/iamkit/sdk/authclient"
	"github.com/Abraxas-365/iamkit/sdk/iamclient"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// Container holds all application dependencies and modules.
type Container struct {
	Config config.Config
	DB     *sqlx.DB
	Redis  *redis.Client
	Auth   *authclient.Client
	Mgmt   *iamclient.Client
	Server *server.Server

	// Modules
	Provider      providermodule.Module
	ProviderKey   providerkeymodule.Module
	Gateway       gatewaymodule.Module
	Usage         usagemodule.Module
	RateLimit     ratelimitmodule.Module
	RoutingConfig routingconfigmodule.Module
	Guardrail     guardrailmodule.Module
	Webhook       webhookmodule.Module
	APIKey        *apikeymodule.Module // nil when IAMKIT_MANAGEMENT_KEY is not set
	Access        *accessmodule.Module // nil when IAMKIT_MANAGEMENT_KEY is not set
}

// New creates and wires the entire application.
func New(cfg config.Config) (*Container, error) {
	c := &Container{Config: cfg}

	if err := c.initInfrastructure(); err != nil {
		return nil, fmt.Errorf("infrastructure: %w", err)
	}
	c.initClients()
	if err := c.initModules(); err != nil {
		return nil, fmt.Errorf("modules: %w", err)
	}
	c.initServer()

	return c, nil
}

func (c *Container) initInfrastructure() error {
	// PostgreSQL
	db, err := sqlx.Connect("postgres", c.Config.Database.DSN())
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	c.DB = db

	// Redis
	c.Redis = redis.NewClient(&redis.Options{
		Addr:     c.Config.Redis.Addr,
		Password: c.Config.Redis.Password,
		DB:       c.Config.Redis.DB,
	})

	return nil
}

func (c *Container) initClients() {
	// IAMKit auth client (token validation, introspection)
	c.Auth = authclient.New(c.Config.IAMKit.BaseURL)

	// IAMKit management client (server-side setup)
	if c.Config.IAMKit.ManagementKey != "" {
		c.Mgmt = iamclient.New(c.Config.IAMKit.BaseURL, c.Config.IAMKit.ManagementKey)
	}
}

func (c *Container) initModules() error {
	c.Provider = providermodule.New(providermodule.Deps{
		DB: c.DB,
	})

	pkm, err := providerkeymodule.New(providerkeymodule.Deps{
		DB:            c.DB,
		EncryptionKey: c.Config.Encryption.Key,
		Providers:     c.Provider.Queries,
	})
	if err != nil {
		return fmt.Errorf("providerkey module: %w", err)
	}
	c.ProviderKey = pkm

	c.Usage = usagemodule.New(usagemodule.Deps{
		DB: c.DB,
	})

	c.RateLimit = ratelimitmodule.New(ratelimitmodule.Deps{
		DB:    c.DB,
		Redis: c.Redis,
	})

	c.RoutingConfig = routingconfigmodule.New(routingconfigmodule.Deps{
		DB: c.DB,
	})

	c.Guardrail = guardrailmodule.New(guardrailmodule.Deps{
		DB: c.DB,
	})

	c.Webhook = webhookmodule.New(webhookmodule.Deps{
		DB: c.DB,
	})

	// API key module (requires IAMKit management key)
	if c.Mgmt != nil {
		m := apikeymodule.New(apikeymodule.Deps{
			IAMKit:        c.Mgmt,
			EnvironmentID: c.Config.IAMKit.EnvironmentID,
			ApplicationID: c.Config.IAMKit.ApplicationID,
			ResourceID:    c.Config.IAMKit.ResourceID,
		})
		c.APIKey = &m

		a := accessmodule.New(accessmodule.Deps{
			IAMKit:         c.Mgmt,
			EnvironmentID:  c.Config.IAMKit.EnvironmentID,
			ResourceID:     c.Config.IAMKit.ResourceID,
			OrganizationID: c.Config.IAMKit.OrganizationID,
		})
		c.Access = &a
	}

	c.Gateway = gatewaymodule.New(gatewaymodule.Deps{
		Models:          c.Provider.ModelQueries,
		Mappings:        c.Provider.MappingQueries,
		Providers:       c.Provider.Queries,
		Fallbacks:       c.Provider.FallbackQueries,
		Keys:            c.ProviderKey.Queries,
		Encryptor:       c.ProviderKey.Encryptor,
		UsageLogger:     c.Usage.Commands,
		RateLimiter:     c.RateLimit.Service,
		Guardrails:      c.Guardrail.Evaluator,
		Webhooks:        c.Webhook.Dispatcher,
		TokenRefresher:  c.ProviderKey.RefreshService,
		RoutingResolver: c.RoutingConfig.Resolver,
		Redis:           c.Redis,
		CacheEnabled:    c.Config.Cache.Enabled,
		CacheTTL:        c.Config.Cache.TTL,
		MetricsEnabled:  c.Config.Metrics.Enabled,
	})

	return nil
}

func (c *Container) initServer() {
	srv := server.New(c.Config.Server.Port)
	app := srv.App()

	// Common middleware
	server.CommonMiddleware(app)

	// Health check (public)
	app.Get("/health", func(ctx *fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	// Prometheus metrics (auth-protected)
	if c.Gateway.Metrics != nil {
		app.Get("/metrics",
			server.AuthMiddleware(c.Auth, c.Config.IAMKit),
			server.RequirePermissions(server.PermMetricsRead),
			adaptor.HTTPHandlerFunc(
				promhttp.HandlerFor(c.Gateway.Metrics.Registry, promhttp.HandlerOpts{}).ServeHTTP,
			),
		)
	}

	// Protected API routes (admin)
	api := app.Group("/api/v1", server.AuthMiddleware(c.Auth, c.Config.IAMKit))

	// Provider module routes (providers, models, mappings, fallbacks)
	c.Provider.HTTP.RegisterRoutes(api)

	// Provider key routes
	c.ProviderKey.HTTP.RegisterRoutes(api.Group("/provider-keys"))

	// Usage log routes
	c.Usage.HTTP.RegisterRoutes(api.Group("/usage"))

	// Rate limit config routes
	c.RateLimit.HTTP.RegisterRoutes(api.Group("/rate-limits"))

	// Routing config routes
	c.RoutingConfig.HTTP.RegisterRoutes(api.Group("/routing-configs"))

	// Guardrail config/rules/violations routes
	c.Guardrail.HTTP.RegisterRoutes(api.Group("/guardrails"))

	// Webhook subscription/delivery routes
	c.Webhook.HTTP.RegisterRoutes(api.Group("/webhooks"))

	// Gateway admin routes (cache invalidation, etc.)
	c.Gateway.HTTP.RegisterAdminRoutes(api.Group("/gateway"))

	// Service account management routes (only available when IAMKit management key is set)
	if c.APIKey != nil {
		c.APIKey.HTTP.RegisterRoutes(api.Group("/service-accounts"))
	}

	// Access management routes: users, roles, role assignments
	if c.Access != nil {
		c.Access.HTTP.RegisterRoutes(api.Group("/access"))
	}

	// Gateway routes (OpenAI-compatible, requires gateway:invoke)
	v1 := app.Group("/v1",
		server.AuthMiddleware(c.Auth, c.Config.IAMKit),
		server.RequirePermissions(server.PermGatewayInvoke),
	)
	c.Gateway.HTTP.RegisterRoutes(v1)
	c.Gateway.HTTP.RegisterModalityRoutes(v1)

	c.Server = srv

	// Start background workers
	c.Webhook.StartWorker()
	c.ProviderKey.RefreshService.StartWorker()
	c.Usage.StartPurgeWorker(time.Hour)
}

// Cleanup releases all resources.
func (c *Container) Cleanup() {
	c.Usage.StopPurgeWorker()
	c.Usage.Close()
	c.Webhook.Stop()
	c.ProviderKey.RefreshService.Stop()
	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			log.Printf("error closing database: %v", err)
		}
	}
	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			log.Printf("error closing redis: %v", err)
		}
	}
}
