package apikeymodule

import (
	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/apikey/adapters/apikeyhttp"
	"github.com/Abraxas-365/freerouter/internal/apikey/adapters/apikeyiamkit"
	"github.com/Abraxas-365/freerouter/internal/apikey/apikeysvc"
	"github.com/Abraxas-365/iamkit/sdk/iamclient"
)

// Deps holds external dependencies for the API key module.
type Deps struct {
	IAMKit        *iamclient.Client // management client (ik_mgmt_...)
	EnvironmentID string            // IAMKit environment UUID
	ApplicationID string            // FreeRouter's IAMKit application UUID
	ResourceID    string            // FreeRouter's IAMKit resource UUID
}

// Module exposes the API key module's public interfaces and HTTP handler.
type Module struct {
	Commands apikey.Commands
	Queries  apikey.Queries
	HTTP     *apikeyhttp.Handler
}

// New wires the API key module: IAMKit store → service → handler.
func New(deps Deps) Module {
	store := apikeyiamkit.New(deps.IAMKit, deps.EnvironmentID)
	svc := apikeysvc.New(store, deps.ApplicationID, deps.ResourceID)

	return Module{
		Commands: svc,
		Queries:  svc,
		HTTP:     apikeyhttp.New(svc, svc),
	}
}
