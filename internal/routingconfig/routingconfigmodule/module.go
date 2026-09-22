package routingconfigmodule

import (
	"github.com/Abraxas-365/freerouter/internal/routingconfig"
	"github.com/Abraxas-365/freerouter/internal/routingconfig/adapters/routingconfighttp"
	"github.com/Abraxas-365/freerouter/internal/routingconfig/adapters/routingconfigpg"
	"github.com/Abraxas-365/freerouter/internal/routingconfig/routingconfigsvc"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the routing config module.
type Deps struct {
	DB *sqlx.DB
}

// Module exposes the routing config module's public interfaces and HTTP handler.
type Module struct {
	Commands routingconfig.Commands
	Queries  routingconfig.Queries
	HTTP     *routingconfighttp.Handler

	// Resolver exposes Resolve for the gateway to call on every request.
	Resolver *routingconfigsvc.Service
}

// New wires the routing config module: repo → service → handler.
func New(deps Deps) Module {
	repo := routingconfigpg.New(deps.DB)
	svc := routingconfigsvc.New(repo)

	return Module{
		Commands: svc,
		Queries:  svc,
		HTTP:     routingconfighttp.New(svc, svc),
		Resolver: svc,
	}
}
