package gatewaymodule

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/gateway"
	"github.com/Abraxas-365/freerouter/internal/gateway/adapters/gatewayhttp"
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/ratelimitsvc"
	"github.com/Abraxas-365/freerouter/internal/routingconfig"
	"github.com/Abraxas-365/freerouter/internal/usage"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/redis/go-redis/v9"
)

// Deps holds external dependencies for the gateway module.
type Deps struct {
	Models          provider.ModelQueries
	Mappings        provider.MappingQueries
	Providers       provider.Queries
	Fallbacks       provider.FallbackQueries
	Keys            providerkey.Queries
	Encryptor       providerkey.TokenEncryptor
	UsageLogger     usage.Commands                  // nil = no usage logging
	RateLimiter     *ratelimitsvc.Service           // nil = no rate limiting
	Guardrails      guardrail.Evaluator             // nil = no guardrail checks
	Webhooks        webhook.Dispatcher              // nil = no webhook events
	TokenRefresher  gatewayhttp.OAuthTokenRefresher // nil = no OAuth refresh
	RoutingResolver routingconfig.Resolver          // nil = always use gateway.StrategyCheapest
	Redis           *redis.Client                   // nil = no response caching
	CacheEnabled    bool
	CacheTTL        time.Duration
	MetricsEnabled  bool
}

// Module exposes the gateway module's public components.
type Module struct {
	Router        *gateway.Router
	Upstream      *gateway.Upstream
	HealthTracker *gateway.KeyHealthTracker
	Cache         *gateway.ResponseCache // nil when caching disabled
	Metrics       *gateway.Metrics       // nil when metrics disabled
	HTTP          *gatewayhttp.Handler
}

// New wires the gateway module.
func New(deps Deps) Module {
	healthTracker := gateway.NewKeyHealthTracker()
	upstream := gateway.NewUpstream()

	router := gateway.NewRouter(
		deps.Models,
		deps.Mappings,
		deps.Providers,
		deps.Keys,
		deps.Encryptor,
		healthTracker,
		deps.Fallbacks,
	)

	var cache *gateway.ResponseCache
	if deps.CacheEnabled {
		cache = gateway.NewResponseCache(deps.Redis, deps.CacheTTL)
	}

	var metrics *gateway.Metrics
	if deps.MetricsEnabled {
		metrics = gateway.NewMetrics()
	}

	handler := gatewayhttp.New(
		router,
		upstream,
		healthTracker,
		deps.Models,
		deps.UsageLogger,
		deps.RateLimiter,
		deps.Guardrails,
		deps.Webhooks,
		deps.TokenRefresher,
		deps.RoutingResolver,
		cache,
		metrics,
	)

	return Module{
		Router:        router,
		Upstream:      upstream,
		HealthTracker: healthTracker,
		Cache:         cache,
		Metrics:       metrics,
		HTTP:          handler,
	}
}
