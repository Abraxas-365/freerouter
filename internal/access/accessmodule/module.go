package accessmodule

import (
	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/access/accesssvc"
	"github.com/Abraxas-365/freerouter/internal/access/adapters/accesshttp"
	"github.com/Abraxas-365/freerouter/internal/access/adapters/accessiamkit"
	"github.com/Abraxas-365/iamkit/sdk/iamclient"
)

// Deps holds external dependencies for the access module.
type Deps struct {
	IAMKit         *iamclient.Client // management client (ik_mgmt_...)
	EnvironmentID  string            // IAMKit environment UUID
	ResourceID     string            // FreeRouter's IAMKit resource UUID
	OrganizationID string            // Default organization UUID
}

// Module exposes the access module's public interfaces and HTTP handler.
type Module struct {
	UserCommands       access.UserCommands
	UserQueries        access.UserQueries
	RoleCommands       access.RoleCommands
	RoleQueries        access.RoleQueries
	AssignmentCommands access.AssignmentCommands
	AssignmentQueries  access.AssignmentQueries
	HTTP               *accesshttp.Handler
}

// New wires the access module: IAMKit store → service → handler.
func New(deps Deps) Module {
	store := accessiamkit.New(deps.IAMKit, deps.EnvironmentID)
	svc := accesssvc.New(store, deps.ResourceID, deps.OrganizationID)

	return Module{
		UserCommands:       svc,
		UserQueries:        svc,
		RoleCommands:       svc,
		RoleQueries:        svc,
		AssignmentCommands: svc,
		AssignmentQueries:  svc,
		HTTP:               accesshttp.New(svc, svc, svc, svc, svc, svc),
	}
}
