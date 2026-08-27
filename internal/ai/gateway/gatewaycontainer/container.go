package gatewaycontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/billing"
	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/gateway/gatewayapi"
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
)

type Deps struct {
	ModelRepo    provider.ModelRepository
	MappingRepo  provider.MappingRepository
	ProviderRepo provider.ProviderRepository
	KeyRepo      providerkey.ProviderKeyRepository
	Encryptor    providerkey.TokenEncryptor
	BillingRepo  billing.BillingRepository
	UsageService *usagesrv.UsageService
}

type Container struct {
	Router   *gateway.Router
	Upstream *gateway.Upstream
	Handlers *gatewayapi.GatewayHandlers
}

func New(deps Deps) *Container {
	router := gateway.NewRouter(
		deps.ModelRepo,
		deps.MappingRepo,
		deps.ProviderRepo,
		deps.KeyRepo,
		deps.Encryptor,
		deps.BillingRepo,
	)
	upstream := gateway.NewUpstream()
	handlers := gatewayapi.NewGatewayHandlers(router, upstream, deps.UsageService)

	return &Container{
		Router:   router,
		Upstream: upstream,
		Handlers: handlers,
	}
}
