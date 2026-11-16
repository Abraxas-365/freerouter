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
	// ExpiresAt is set for keys FreeRouter issues itself (none/token modes);
	// nil for keys that never expire and for IAMKit service accounts.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
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
	ID     string `json:"id"`
	Secret string `json:"secret"`
	// ExpiresAt is the zero time for a key that never expires (none/token
	// modes; IAMKit returns a far-future time instead).
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
	// CallerExpiresAt is when the caller's own credential expires (nil when
	// it has no fixed end), set by the transport like CallerPermissions.
	CallerExpiresAt *time.Time `json:"-"`
}

// Validate checks the create request and normalises permissions.
func (c *CreateServiceAccount) Validate() error {
	if c.Name == "" {
		return errx.Validation("name is required")
	}
	if _, err := ExpiryFrom(c.ExpiresIn, time.Now()); err != nil {
		return err
	}
	// Default to gateway:invoke when no permissions specified (a copy: the
	// store and callers may keep the slice).
	if len(c.Permissions) == 0 {
		c.Permissions = append([]string(nil), server.DefaultPermissions...)
		return nil
	}
	for _, p := range c.Permissions {
		if !server.ValidPermissions[p] {
			return errx.Validation(fmt.Sprintf("unknown permission: %q", p))
		}
	}
	return nil
}

// Expiry bounds of a key, the same in every auth mode (IAMKit's for
// service accounts).
const (
	DefaultExpiry = 24 * time.Hour
	MinExpiry     = time.Hour
	MaxExpiry     = 8760 * time.Hour
	// NeverExpires is the expires_in value of a key that does not expire.
	NeverExpires = "never"
)

// ExpiryFrom is when a key created at now with expires_in expires: nil for
// "never", now+24h when empty, else a Go duration between 1h and 8760h.
func ExpiryFrom(expiresIn string, now time.Time) (*time.Time, error) {
	switch expiresIn {
	case NeverExpires:
		return nil, nil
	case "":
		at := now.Add(DefaultExpiry)
		return &at, nil
	}
	d, err := time.ParseDuration(expiresIn)
	switch {
	case err != nil:
		return nil, errx.Validation(fmt.Sprintf("expires_in must be a Go duration (e.g. 720h) or %q", NeverExpires))
	case d < MinExpiry:
		return nil, errx.Validation("expires_in must be at least 1h")
	case d > MaxExpiry:
		return nil, errx.Validation("expires_in must be at most 8760h")
	}
	at := now.Add(d)
	return &at, nil
}

// AuthorizeGrant ensures the caller holds every permission it is granting,
// so service-accounts:write cannot mint a key more powerful than its holder,
// and that a caller whose credential expires cannot mint a key outliving it
// (a short-lived key must not buy itself persistence). Call after Validate
// (which fills in the default permissions). A caller without permissions
// can grant nothing.
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
	if c.CallerExpiresAt != nil {
		at, err := ExpiryFrom(c.ExpiresIn, time.Now())
		if err != nil {
			return err
		}
		if at == nil || at.After(*c.CallerExpiresAt) {
			return errx.Forbidden("cannot create a key that outlives your own (expires " +
				c.CallerExpiresAt.UTC().Format(time.RFC3339) + ")")
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
