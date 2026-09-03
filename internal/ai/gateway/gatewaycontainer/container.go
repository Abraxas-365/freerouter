package gatewaycontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/gateway/gatewayapi"
	"github.com/Abraxas-365/freerouter/internal/ai/gateway/gatewayinfra"
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails/guardrailssrv"
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	DB             *sqlx.DB
	ModelRepo      provider.ModelRepository
	MappingRepo    provider.MappingRepository
	ProviderRepo   provider.ProviderRepository
	FallbackRepo   provider.FallbackRepository
	KeyRepo        providerkey.ProviderKeyRepository
	Encryptor      providerkey.TokenEncryptor
	BillingRepo    billing.BillingRepository
	BillingService *billingsrv.BillingService
	UsageService   *usagesrv.UsageService
	GuardrailsSvc  *guardrailssrv.GuardrailsService
	Redis          *redis.Client
}

type Container struct {
	Router        *gateway.Router
	Upstream      *gateway.Upstream
	Handlers      *gatewayapi.GatewayHandlers
	HealthTracker *gateway.KeyHealthTracker
	RateLimiter   *gateway.RateLimiter
	Cache         *gateway.ResponseCache
	Metrics       *gateway.Metrics
}

func New(deps Deps) *Container {
	healthTracker := gateway.NewKeyHealthTracker()
	rateLimitRepo := gatewayinfra.NewPostgresRateLimitConfigRepository(deps.DB)
	rateLimiter := gateway.NewRateLimiter(deps.Redis, gateway.DefaultRateLimitConfig(), rateLimitRepo)
	cache := gateway.NewResponseCache(deps.Redis, gateway.DefaultCacheTTL)
	metrics := gateway.NewMetrics()

	router := gateway.NewRouter(
		deps.ModelRepo,
		deps.MappingRepo,
		deps.ProviderRepo,
		deps.KeyRepo,
		deps.Encryptor,
		deps.BillingRepo,
		healthTracker,
		deps.FallbackRepo,
	)

	// Wire routing config repo for per-tenant strategy
	routingConfigRepo := gatewayinfra.NewPostgresRoutingConfigRepository(deps.DB)
	router.SetRoutingConfigRepo(routingConfigRepo)

	upstream := gateway.NewUpstream()
	handlers := gatewayapi.NewGatewayHandlers(
		router, upstream, deps.UsageService, deps.BillingService,
		deps.ModelRepo, deps.MappingRepo, healthTracker,
		deps.GuardrailsSvc, rateLimiter, cache, metrics,
	)
	handlers.SetRoutingConfigRepo(routingConfigRepo)

	return &Container{
		Router:        router,
		Upstream:      upstream,
		Handlers:      handlers,
		HealthTracker: healthTracker,
		RateLimiter:   rateLimiter,
		Cache:         cache,
		Metrics:       metrics,
	}
}
