package providermodule

import (
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/provider/adapters/providerhttp"
	"github.com/Abraxas-365/freerouter/internal/provider/adapters/providerpg"
	"github.com/Abraxas-365/freerouter/internal/provider/providersvc"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the provider module.
type Deps struct {
	DB *sqlx.DB
}

// Module exposes the provider module's public interfaces and HTTP handler.
type Module struct {
	Commands         provider.Commands
	Queries          provider.Queries
	ModelCommands    provider.ModelCommands
	ModelQueries     provider.ModelQueries
	MappingCommands  provider.MappingCommands
	MappingQueries   provider.MappingQueries
	FallbackCommands provider.FallbackCommands
	FallbackQueries  provider.FallbackQueries
	HTTP             *providerhttp.Handler
}

// New wires the provider module: repos → service → handler.
func New(deps Deps) Module {
	providerRepo := providerpg.NewProvider(deps.DB)
	modelRepo := providerpg.NewModel(deps.DB)
	mappingRepo := providerpg.NewMapping(deps.DB)
	fallbackRepo := providerpg.NewFallback(deps.DB)

	svc := providersvc.New(providerRepo, modelRepo, mappingRepo, fallbackRepo)

	return Module{
		Commands:         svc,
		Queries:          svc,
		ModelCommands:    svc,
		ModelQueries:     svc,
		MappingCommands:  svc,
		MappingQueries:   svc,
		FallbackCommands: svc,
		FallbackQueries:  svc,
		HTTP: providerhttp.New(
			svc, svc, // provider cmd/qry
			svc, svc, // model cmd/qry
			svc, svc, // mapping cmd/qry
			svc, svc, // fallback cmd/qry
		),
	}
}
