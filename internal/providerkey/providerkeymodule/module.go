package providerkeymodule

import (
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/providerkey/adapters/providerkeyhttp"
	"github.com/Abraxas-365/freerouter/internal/providerkey/adapters/providerkeyinfra"
	"github.com/Abraxas-365/freerouter/internal/providerkey/adapters/providerkeypg"
	"github.com/Abraxas-365/freerouter/internal/providerkey/providerkeysvc"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the provider key module.
type Deps struct {
	DB            *sqlx.DB
	EncryptionKey string           // 32-byte hex-encoded key
	Providers     provider.Queries // to verify provider exists on create
}

// Module exposes the provider key module's public interfaces and HTTP handler.
type Module struct {
	Commands       providerkey.Commands
	Queries        providerkey.Queries
	Encryptor      providerkey.TokenEncryptor
	RefreshService *providerkeysvc.RefreshService
	HTTP           *providerkeyhttp.Handler
}

// New wires the provider key module: encryptor + repo → service → handler.
func New(deps Deps) (Module, error) {
	encryptor, err := providerkeyinfra.NewEncryptor(deps.EncryptionKey)
	if err != nil {
		return Module{}, err
	}

	repo := providerkeypg.New(deps.DB)
	svc := providerkeysvc.New(repo, encryptor, deps.Providers)

	// Create refresh service with provider-specific refreshers.
	refreshSvc := providerkeysvc.NewRefreshService(
		repo, encryptor, deps.Providers,
		[]providerkey.TokenRefresher{
			providerkeyinfra.NewAnthropicRefresher(),
			providerkeyinfra.NewOpenAIRefresher(),
		},
	)

	return Module{
		Commands:       svc,
		Queries:        svc,
		Encryptor:      encryptor,
		RefreshService: refreshSvc,
		HTTP:           providerkeyhttp.New(svc, svc),
	}, nil
}
