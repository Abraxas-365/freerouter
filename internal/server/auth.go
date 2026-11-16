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

// IAMKitClient is the IAMKit surface the iam mode needs.
// *authclient.Client satisfies it; tests inject a fake.
type IAMKitClient interface {
	Introspect(ctx context.Context, token, issuer, audience, environment, application, resource string) (*authclient.Claims, error)
	MachineToken(ctx context.Context, secret string) (authclient.TokenPair, error)
}

// IAM is FREEROUTER_AUTH=iam: access tokens are validated by IAMKit online
// introspection (live revocation) against the trusted boundaries in
// Config, and the principal carries the token's subject and permissions.
//
// Accepts credentials either as "Authorization: Bearer <token>" or as
// "X-Api-Key: <token>" (Anthropic SDK / rness anthropic-adapter convention).
//
// Service-account secrets (ik_svc_...) are transparently exchanged for
// short-lived JWTs via IAMKit's MachineToken endpoint and cached in-memory
// (keyed by a hash, never the raw secret), so callers can use the secret
// directly as a Bearer token — no manual exchange step required
// (OpenRouter-style DX). The exchanged JWT is still introspected on every
// request, so revocation applies immediately.
type IAM struct {
	Client IAMKitClient
	Config config.IAMKit
}

var _ Authenticator = IAM{}

// Authenticate returns the middleware. Build it once and share it between
// route groups: it owns the token cache.
func (a IAM) Authenticate() fiber.Handler {
	cache := newTokenCache(tokenCacheMargin, tokenCacheSweep)
	cfg := a.Config

	validate := func(ctx context.Context, raw string) (*authclient.Claims, error) {
		token := raw

		// Transparently exchange ik_svc_ credentials for a JWT.
		if strings.HasPrefix(raw, svcTokenPrefix) {
			key := secretKey(raw)
			if cached := cache.Get(key); cached != "" {
				token = cached
			} else {
				exchangeCtx, cancel := context.WithTimeout(ctx, iamCallTimeout)
				pair, err := a.Client.MachineToken(exchangeCtx, raw)
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
		claims, err := a.Client.Introspect(
			introspectCtx, token,
			cfg.JWTIssuer,
			cfg.Audience,
			cfg.EnvironmentID,
			cfg.ApplicationID,
			cfg.ResourceID,
		)
		if err != nil || claims == nil {
			return nil, errx.Unauthorized("invalid or expired token")
		}
		return claims, nil
	}

	return func(c *fiber.Ctx) error {
		normalizeAPIKeyHeader(c)
		raw := bearer(c)
		if raw == "" {
			return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid bearer token")
		}
		claims, err := validate(c.UserContext(), raw)
		if err != nil {
			return authError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
		}
		SetPrincipal(c, Principal{Subject: claims.Subject, Permissions: claims.Permissions})
		return c.Next()
	}
}

// Session: none — the console signs in against IAMKit directly.
func (IAM) Session() SessionRoutes { return SessionRoutes{} }

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
