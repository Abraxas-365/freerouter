package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	// iamCallTimeout bounds each IAMKit call made while authenticating a
	// request. IAMKit being slow or down fails the request closed (401)
	// instead of hanging it.
	iamCallTimeout = 5 * time.Second
)

// Authenticator is the IAMKit surface the auth middleware needs.
// *authclient.Client satisfies it; tests inject a fake.
type Authenticator interface {
	Introspect(ctx context.Context, token, issuer, audience, environment, application, resource string) (*authclient.Claims, error)
	MachineToken(ctx context.Context, secret string) (authclient.TokenPair, error)
}

// AuthMiddleware returns a Fiber middleware that validates access tokens via
// IAMKit online introspection (live revocation) against the trusted
// boundaries in cfg, and stores claims on the request context. Build it
// once and share it between route groups: it owns the token cache.
//
// Accepts credentials either as "Authorization: Bearer <token>" or as
// "X-Api-Key: <token>" (Anthropic SDK / rness anthropic-adapter convention).
// When only X-Api-Key is present, it is normalized into an Authorization
// bearer header before delegating to the underlying validator.
//
// Service-account secrets (ik_svc_...) are transparently exchanged for
// short-lived JWTs via IAMKit's MachineToken endpoint and cached in-memory
// (keyed by a hash, never the raw secret), so callers can use the secret
// directly as a Bearer token — no manual exchange step required
// (OpenRouter-style DX). The exchanged JWT is still introspected on every
// request, so revocation applies immediately.
func AuthMiddleware(auth Authenticator, cfg config.IAMKit) fiber.Handler {
	cache := newTokenCache(tokenCacheMargin, tokenCacheSweep)

	validate := func(ctx context.Context, raw string) (*authclient.Claims, error) {
		token := raw

		// Transparently exchange ik_svc_ credentials for a JWT.
		if strings.HasPrefix(raw, svcTokenPrefix) {
			key := secretKey(raw)
			if cached := cache.Get(key); cached != "" {
				token = cached
			} else {
				exchangeCtx, cancel := context.WithTimeout(ctx, iamCallTimeout)
				pair, err := auth.MachineToken(exchangeCtx, raw)
				cancel()
				if err != nil {
					return nil, errx.Unauthorized("invalid service account credential")
				}
				cache.Set(key, pair.AccessToken, pair.ExpiresIn)
				token = pair.AccessToken
			}
		}

		introspectCtx, cancel := context.WithTimeout(ctx, iamCallTimeout)
		defer cancel()
		claims, err := auth.Introspect(
			introspectCtx, token,
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

// secretKey is the cache key for a service-account secret: its SHA-256, so
// raw credentials are not kept in memory longer than the request.
func secretKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
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
