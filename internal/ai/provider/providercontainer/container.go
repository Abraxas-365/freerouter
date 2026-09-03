package providercontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/provider/providerapi"
	"github.com/Abraxas-365/freerouter/internal/ai/provider/providerinfra"
	"github.com/Abraxas-365/freerouter/internal/ai/provider/providersrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	ProviderRepo provider.ProviderRepository
	ModelRepo    provider.ModelRepository
	MappingRepo  provider.MappingRepository
	FallbackRepo provider.FallbackRepository
	Service      *providersrv.ProviderService
	Handlers     *providerapi.ProviderHandlers
}

func New(db *sqlx.DB) *Container {
	providerRepo := providerinfra.NewPostgresProviderRepository(db)
	modelRepo := providerinfra.NewPostgresModelRepository(db)
	mappingRepo := providerinfra.NewPostgresMappingRepository(db)
	fallbackRepo := providerinfra.NewPostgresFallbackRepository(db)

	service := providersrv.NewProviderService(providerRepo, modelRepo, mappingRepo, fallbackRepo)
	handlers := providerapi.NewProviderHandlers(service)

	return &Container{
		ProviderRepo: providerRepo,
		ModelRepo:    modelRepo,
		MappingRepo:  mappingRepo,
		FallbackRepo: fallbackRepo,
		Service:      service,
		Handlers:     handlers,
	}
}
