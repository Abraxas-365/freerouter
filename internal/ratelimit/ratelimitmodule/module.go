package ratelimitmodule

import (
	"github.com/Abraxas-365/freerouter/internal/ratelimit"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/adapters/ratelimithttp"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/adapters/ratelimitpg"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/adapters/ratelimitredis"
	"github.com/Abraxas-365/freerouter/internal/ratelimit/ratelimitsvc"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// Deps holds external dependencies for the rate limit module.
type Deps struct {
	DB    *sqlx.DB
	Redis *redis.Client
}

// Module exposes the rate limit module's public interfaces and HTTP handler.
type Module struct {
	Commands ratelimit.Commands
	Queries  ratelimit.Queries
	HTTP     *ratelimithttp.Handler

	// Service exposes Check/Release for the gateway to call.
	Service *ratelimitsvc.Service
}

// New wires the rate limit module: repo + redis limiter → service → handler.
func New(deps Deps) Module {
	repo := ratelimitpg.New(deps.DB)
	limiter := ratelimitredis.New(deps.Redis)
	svc := ratelimitsvc.New(repo, limiter)

	return Module{
		Commands: svc,
		Queries:  svc,
		HTTP:     ratelimithttp.New(svc, svc),
		Service:  svc,
	}
}
