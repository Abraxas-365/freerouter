package server

import (
	"context"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/iamkit/sdk/authclient"
	"github.com/Abraxas-365/iamkit/sdk/authclient/fiberauth"
	"github.com/gofiber/fiber/v2"
)

const (
	// svcTokenPrefix identifies IAMKit service-account credentials.
	svcTokenPrefix = "ik_svc_"

	// tokenCacheMargin is subtracted from the JWT TTL before caching so
	// that we refresh slightly before the real expiry.
	tokenCacheMargin = 30 * time.Second

	// tokenCacheSweep is how often the background sweeper evicts expired
	// entries from the service-account token cache.
	tokenCacheSweep = 5 * time.Minute
)

// AuthMiddleware returns a Fiber middleware that validates JWTs via IAMKit
// online introspection and stores claims on the request context.
//
// Accepts credentials either as "Authorization: Bearer <token>" or as
// "X-Api-Key: <token>" (Anthropic SDK / rness anthropic-adapter convention).
// When only X-Api-Key is present, it is normalized into an Authorization
// bearer header before delegating to the underlying validator.
//
// Service-account secrets (ik_svc_...) are transparently exchanged for
// short-lived JWTs via IAMKit's MachineToken endpoint and cached in-memory,
// so callers can use the secret directly as a Bearer token — no manual
// exchange step required (OpenRouter-style DX).
func AuthMiddleware(auth *authclient.Client, cfg config.IAMKit) fiber.Handler {
	cache := newTokenCache(tokenCacheMargin, tokenCacheSweep)

	validate := func(ctx context.Context, raw string) (*authclient.Claims, error) {
		token := raw

		// Transparently exchange ik_svc_ credentials for a JWT.
		if strings.HasPrefix(raw, svcTokenPrefix) {
			if cached := cache.Get(raw); cached != "" {
				token = cached
			} else {
				pair, err := auth.MachineToken(ctx, raw)
				if err != nil {
					return nil, errx.Unauthorized("invalid service account credential")
				}
				cache.Set(raw, pair.AccessToken, pair.ExpiresIn)
				token = pair.AccessToken
			}
		}

		claims, err := auth.Introspect(
			ctx, token,
			cfg.JWTIssuer,
			cfg.Audience,
			cfg.EnvironmentID,
			cfg.ApplicationID,
			cfg.ResourceID,
		)
		if err != nil {
			return nil, errx.Unauthorized("invalid or expired token")
		}
		return claims, nil
	}
	next := fiberauth.Authenticate(validate)
	return func(c *fiber.Ctx) error {
		normalizeAPIKeyHeader(c)
		return next(c)
	}
}

// normalizeAPIKeyHeader rewrites "X-Api-Key: <token>" into
// "Authorization: Bearer <token>" when no Authorization header is already
// present. This lets clients built against Anthropic's SDK conventions
// (x-api-key auth) call FreeRouter without modification.
func normalizeAPIKeyHeader(c *fiber.Ctx) {
	if strings.TrimSpace(c.Get("Authorization")) != "" {
		return
	}
	apiKey := strings.TrimSpace(c.Get("X-Api-Key"))
	if apiKey == "" {
		return
	}
	c.Request().Header.Set("Authorization", "Bearer "+apiKey)
}

// RequirePermissions returns middleware that checks the authenticated claims
// for the specified permissions. Returns 403 if any are missing.
func RequirePermissions(perms ...string) fiber.Handler {
	return fiberauth.RequirePermissions(perms...)
}

// Claims extracts the authenticated IAMKit claims from the Fiber context.
// Returns nil if no claims are set (unauthenticated route).
func Claims(c *fiber.Ctx) *authclient.Claims {
	return fiberauth.Claims(c)
}
