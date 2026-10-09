package apikey

import (
	"fmt"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
)

// ServiceAccount represents a service-account-backed credential for gateway
// or admin access. Stored and managed by IAMKit; FreeRouter only proxies CRUD.
type ServiceAccount struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ApplicationID string   `json:"application_id"`
	ResourceID    string   `json:"resource_id"`
	Permissions   []string `json:"permissions"`
	ExpiresIn     string   `json:"expires_in,omitempty"`
}

// Application is an IAMKit application that a service account can belong to
// (i.e. "which service is this credential for"). Applications are managed in
// IAMKit; FreeRouter only lists them so the caller can pick one on create.
type Application struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// ServiceAccountCredential is returned once on creation — contains the raw secret.
type ServiceAccountCredential struct {
	ID        string    `json:"id"`
	Secret    string    `json:"secret"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateServiceAccount is the input for creating a new service account.
type CreateServiceAccount struct {
	Name          string   `json:"name"`
	ApplicationID string   `json:"application_id,omitempty"` // IAMKit application UUID; defaults to FreeRouter's own application when omitted
	Permissions   []string `json:"permissions"`
	ExpiresIn     string   `json:"expires_in,omitempty"` // Go duration string, e.g. "8760h"

	// CallerPermissions are the permissions of the authenticated caller,
	// set by the transport from the token, never from the request body.
	CallerPermissions []string `json:"-"`
}

// Validate checks the create request and normalises permissions.
func (c *CreateServiceAccount) Validate() error {
	if c.Name == "" {
		return errx.Validation("name is required")
	}
	// Default to gateway:invoke when no permissions specified.
	if len(c.Permissions) == 0 {
		c.Permissions = server.DefaultPermissions
		return nil
	}
	for _, p := range c.Permissions {
		if !server.ValidPermissions[p] {
			return errx.Validation(fmt.Sprintf("unknown permission: %q", p))
		}
	}
	return nil
}

// AuthorizeGrant ensures the caller holds every permission it is granting,
// so service-accounts:write cannot mint a key more powerful than its holder.
// Call after Validate (which fills in the default permissions). A caller
// without permissions can grant nothing.
func (c CreateServiceAccount) AuthorizeGrant() error {
	held := make(map[string]bool, len(c.CallerPermissions))
	for _, p := range c.CallerPermissions {
		held[p] = true
	}
	for _, p := range c.Permissions {
		if !held[p] {
			return errx.Forbidden(fmt.Sprintf("cannot grant permission you do not hold: %q", p))
		}
	}
	return nil
}

// ── Deprecated aliases (to ease transition) ─────────────────────────
// TODO: Remove after renaming all call sites.

// APIKey is the legacy alias for ServiceAccount.
type APIKey = ServiceAccount

// APIKeyCredential is the legacy alias for ServiceAccountCredential.
type APIKeyCredential = ServiceAccountCredential

// CreateAPIKey is the legacy alias for CreateServiceAccount.
type CreateAPIKey = CreateServiceAccount
