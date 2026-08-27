package aicontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/gateway/gatewaycontainer"
	"github.com/Abraxas-365/freerouter/internal/ai/provider/providercontainer"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey/providerkeycontainer"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey/providerkeyinfra"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagecontainer"
	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/logx"
	"github.com/jmoiron/sqlx"
)

type Deps struct {
	DB             *sqlx.DB
	Cfg            *config.Config
	BillingRepo    billing.BillingRepository
	BillingService *billingsrv.BillingService
}

type Container struct {
	Provider    *providercontainer.Container
	ProviderKey *providerkeycontainer.Container
	Gateway     *gatewaycontainer.Container
	Usage       *usagecontainer.Container
}

func New(deps Deps) *Container {
	logx.Info("Initializing AI container...")

	c := &Container{}

	// 1. Provider registry (no deps on other AI modules)
	c.Provider = providercontainer.New(deps.DB)
	logx.Info("  Provider registry initialized")

	// 2. Encryption for provider keys
	encryptor, err := providerkeyinfra.NewAESTokenEncryptor(deps.Cfg.AI.EncryptionKey)
	if err != nil {
		logx.Fatalf("Failed to initialize token encryptor: %v", err)
	}

	// 3. Provider keys (depends on provider repo + encryptor)
	c.ProviderKey = providerkeycontainer.New(deps.DB, c.Provider.ProviderRepo, encryptor)
	logx.Info("  Provider key management initialized")

	// 4. Usage logging (no deps on other AI modules)
	c.Usage = usagecontainer.New(deps.DB, deps.Cfg.AI.UsageBufferSize)
	logx.Info("  Usage logging initialized")

	// 5. Gateway (depends on provider, providerkey, billing, usage)
	c.Gateway = gatewaycontainer.New(gatewaycontainer.Deps{
		ModelRepo:      c.Provider.ModelRepo,
		MappingRepo:    c.Provider.MappingRepo,
		ProviderRepo:   c.Provider.ProviderRepo,
		KeyRepo:        c.ProviderKey.KeyRepo,
		Encryptor:      c.ProviderKey.Encryptor,
		BillingRepo:    deps.BillingRepo,
		BillingService: deps.BillingService,
		UsageService:   c.Usage.Service,
	})
	logx.Info("  Gateway initialized")

	logx.Info("AI container initialized")
	return c
}

func (c *Container) Close() {
	if c.Usage != nil {
		c.Usage.Close()
	}
}
