package server

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Principal is who is calling, as established by the Authenticator of the
// run mode. Every request behind an Authenticator carries one; handlers and
// guards read it (Caller, RequirePermissions) and never look at the mode.
type Principal struct {
	// Subject names the caller for rate limits, routing configs and cache
	// keys: "local" (none mode), "admin" (token mode's admin token), a
	// gateway key id, or an IAMKit subject (user or service account).
	Subject string
	// Permissions are the freerouter:* permissions the caller holds: the
	// key's grants, the IAMKit token's, or LocalPermissions for the
	// single-owner principals of the none and token modes.
	Permissions []string
	// ExpiresAt is when the credential stops working, for credentials that
	// grant keys (fr_ gateway keys): a key may not mint a key outliving it.
	// Nil for principals without a fixed end (owner, admin token, IAMKit).
	ExpiresAt *time.Time `json:",omitempty"`
}

// Can reports whether the principal holds perm.
func (p Principal) Can(perm string) bool {
	if perm == "" {
		return false
	}
	for _, have := range p.Permissions {
		if have == perm {
			return true
		}
	}
	return false
}

type principalKey struct{}

// SetPrincipal makes p the principal of the request.
func SetPrincipal(c *fiber.Ctx, p Principal) {
	c.Locals(principalKey{}, p)
}

// Caller returns the request's principal; ok is false on a route that is
// not behind an Authenticator.
func Caller(c *fiber.Ctx) (Principal, bool) {
	p, ok := c.Locals(principalKey{}).(Principal)
	return p, ok
}

// RequirePermissions answers 401 without a principal and 403 unless the
// principal holds every one of perms.
func RequirePermissions(perms ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		p, ok := Caller(c)
		if !ok {
			return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
		}
		for _, perm := range perms {
			if !p.Can(perm) {
				return authError(c, fiber.StatusForbidden, "FORBIDDEN", "insufficient permissions")
			}
		}
		return c.Next()
	}
}

// Authenticator establishes the principal of a request. It is the inbound
// auth port; bootstrap picks the adapter of the run mode (Open, Tokens,
// IAM) and nothing else knows the mode.
type Authenticator interface {
	// Authenticate is the middleware of every protected group (/api/v1,
	// /v1, /metrics): it answers 401/403 or sets the principal and calls
	// Next. Routes then check permissions with RequirePermissions.
	Authenticate() fiber.Handler
	// Session is /api/v1/session for the console. Nil handlers are not
	// mounted (iam mode: the console signs in against IAMKit).
	Session() SessionRoutes
}

// SessionRoutes are the browser session endpoints of a mode: Current is
// GET /api/v1/session (never 401), Login POST, Logout DELETE.
type SessionRoutes struct {
	Current, Login, Logout fiber.Handler
}

// KeyResolver turns a locally issued gateway key (fr_...) into its
// principal; unknown, revoked or expired keys are errx.Unauthorized.
type KeyResolver interface {
	ResolveKey(ctx context.Context, secret string) (Principal, error)
}

// LocalKeyPrefix marks the gateway keys FreeRouter issues itself (none and
// token modes); IAMKit service-account secrets start with ik_svc_.
const LocalKeyPrefix = "fr_"

// bearer returns the request's bearer token ("" when absent or malformed:
// exactly "Bearer <token>", like IAMKit's fiberauth). Call
// normalizeAPIKeyHeader first so X-Api-Key counts as the bearer.
func bearer(c *fiber.Ctx) string {
	f := strings.Fields(c.Get("Authorization"))
	if len(f) != 2 || !strings.EqualFold(f[0], "Bearer") {
		return ""
	}
	return f[1]
}

// authError writes the error envelope IAMKit's fiberauth middleware uses,
// so callers see one shape for authentication failures in every mode.
func authError(c *fiber.Ctx, status int, code, message string) error {
	c.Set("Cache-Control", "no-store")
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{"code": code, "message": message},
	})
}
