package apikeymodule

import (
	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/apikey/adapters/apikeyhttp"
	"github.com/Abraxas-365/freerouter/internal/apikey/adapters/apikeyiamkit"
	"github.com/Abraxas-365/freerouter/internal/apikey/adapters/apikeypg"
	"github.com/Abraxas-365/freerouter/internal/apikey/apikeysvc"
	"github.com/Abraxas-365/freerouter/internal/iamx"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/jmoiron/sqlx"
)

// Deps holds external dependencies for the API key module.
type Deps struct {
	IAMKit        *iamx.Client // backend service account on the IAM resource
	ApplicationID string       // FreeRouter's IAMKit application UUID
	ResourceID    string       // FreeRouter's IAMKit resource UUID
}

// LocalDeps holds the dependencies of the module without IAMKit.
type LocalDeps struct {
	DB *sqlx.DB
}

// Module exposes the API key module's public interfaces and HTTP handler.
type Module struct {
	Commands apikey.Commands
	Queries  apikey.Queries
	HTTP     *apikeyhttp.Handler
	// Keys resolves presented keys to principals: local keys only, nil in
	// iam mode (IAMKit authenticates its service accounts).
	Keys server.KeyResolver
}

// New wires the API key module for iam mode: IAMKit store → service → handler.
func New(deps Deps) Module {
	store := apikeyiamkit.New(deps.IAMKit)
	svc := apikeysvc.New(store, deps.ApplicationID, deps.ResourceID)

	return Module{
		Commands: svc,
		Queries:  svc,
		HTTP:     apikeyhttp.New(svc, svc),
	}
}

// NewLocal wires the API key module for the none and token modes: gateway
// keys issued and stored by FreeRouter (PostgreSQL) → service → handler,
// plus the resolver the Authenticator checks presented keys with.
func NewLocal(deps LocalDeps) Module {
	store := apikeypg.New(deps.DB)
	svc := apikeysvc.New(store, "", "")

	return Module{
		Commands: svc,
		Queries:  svc,
		HTTP:     apikeyhttp.New(svc, svc),
		Keys:     apikeysvc.NewResolver(store),
	}
}
