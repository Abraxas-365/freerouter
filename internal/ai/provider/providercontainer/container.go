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
	Service      *providersrv.ProviderService
	Handlers     *providerapi.ProviderHandlers
}

func New(db *sqlx.DB) *Container {
	providerRepo := providerinfra.NewPostgresProviderRepository(db)
	modelRepo := providerinfra.NewPostgresModelRepository(db)
	mappingRepo := providerinfra.NewPostgresMappingRepository(db)

	service := providersrv.NewProviderService(providerRepo, modelRepo, mappingRepo)
	handlers := providerapi.NewProviderHandlers(service)

	return &Container{
		ProviderRepo: providerRepo,
		ModelRepo:    modelRepo,
		MappingRepo:  mappingRepo,
		Service:      service,
		Handlers:     handlers,
	}
}
