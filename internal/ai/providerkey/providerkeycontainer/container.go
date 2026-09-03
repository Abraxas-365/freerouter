package providerkeycontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey/providerkeyapi"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey/providerkeyinfra"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey/providerkeysrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	KeyRepo   providerkey.ProviderKeyRepository
	Encryptor providerkey.TokenEncryptor
	Service   *providerkeysrv.ProviderKeyService
	Handlers  *providerkeyapi.ProviderKeyHandlers
}

func New(db *sqlx.DB, providerRepo provider.ProviderRepository, encryptor providerkey.TokenEncryptor) *Container {
	keyRepo := providerkeyinfra.NewPostgresProviderKeyRepository(db)

	service := providerkeysrv.NewProviderKeyService(keyRepo, providerRepo, encryptor)
	handlers := providerkeyapi.NewProviderKeyHandlers(service)

	return &Container{
		KeyRepo:   keyRepo,
		Encryptor: encryptor,
		Service:   service,
		Handlers:  handlers,
	}
}
