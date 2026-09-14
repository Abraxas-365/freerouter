package auth

import (
	"github.com/Abraxas-365/freerouter/internal/iam/tenant"
	"github.com/Abraxas-365/freerouter/internal/iam/user"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/iam"
	"github.com/Abraxas-365/freerouter/internal/iam/apikey"
	"github.com/Abraxas-365/freerouter/internal/iam/apikey/apikeysrv"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type UnifiedAuthMiddleware struct {
	apiKeyService *apikeysrv.APIKeyService
	tokenService  TokenService
	userRepo      user.UserRepository
	tenantRepo    tenant.TenantRepository
	cookieName    string
	sessionRepo   SessionRepository
	scopeResolver ScopeResolver
}

func NewAPIKeyMiddleware(
	apiKeyService *apikeysrv.APIKeyService,
	tokenService TokenService,
	userRepo user.UserRepository, tenantRepo tenant.TenantRepository, cookieName string, sessionRepo SessionRepository, scopeResolver ScopeResolver,
) *UnifiedAuthMiddleware {
	return &UnifiedAuthMiddleware{
		sessionRepo: sessionRepo, scopeResolver: scopeResolver,
		apiKeyService: apiKeyService,
		tokenService:  tokenService, userRepo: userRepo, tenantRepo: tenantRepo, cookieName: cookieName,
	}
}

func (am *UnifiedAuthMiddleware) Authenticate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := extractAPIKey(c)
		if apiKey != "" {
			return am.authenticateAPIKey(c, apiKey)
		}

		return am.authenticateJWT(c)
	}
}

func (am *UnifiedAuthMiddleware) authenticateAPIKey(c *fiber.Ctx, keyString string) error {
	key, err := am.apiKeyService.ValidateAPIKey(c.Context(), keyString)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	authContext := &kernel.AuthContext{
		Actor:         kernel.NewAPIKeyActor(kernel.NewAPIKeyID(key.ID)),
		TenantID:      key.TenantID,
		Scopes:        key.Scopes,
		AllowedModels: key.AllowedModels,
		WalletID:      key.WalletID,
	}

	c.Locals("auth", authContext)
	c.Locals("api_key_id", key.ID)

	return c.Next()
}

func (am *UnifiedAuthMiddleware) authenticateJWT(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	var token string

	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" && parts[1] != "" {
			token = parts[1]
		}
	}

	if token == "" {
		token = c.Cookies(am.cookieName)
	}

	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": iam.ErrUnauthorized().Error(),
		})
	}

	claims, err := am.tokenService.ValidateAccessToken(token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	u, err := am.userRepo.FindByID(c.Context(), claims.UserID, claims.TenantID)
	if err != nil || u == nil || !u.CanLogin() || u.CredentialVersion != claims.CredentialVersion {
		return iam.ErrUnauthorized()
	}
	t, err := am.tenantRepo.FindByID(c.Context(), claims.TenantID)
	if err != nil || t == nil || !t.IsActive() {
		return iam.ErrUnauthorized()
	}
	if claims.SessionID == "" {
		return iam.ErrUnauthorized()
	}
	session, err := am.sessionRepo.FindSession(c.Context(), claims.SessionID)
	if err != nil || session == nil || session.UserID != claims.UserID || session.TenantID != claims.TenantID || !session.ExpiresAt.After(time.Now()) {
		return iam.ErrUnauthorized()
	}
	effective, err := am.scopeResolver.GetEffectiveScopes(c.Context(), claims.UserID, claims.TenantID)
	if err != nil {
		return iam.ErrUnauthorized()
	}
	authContext := &kernel.AuthContext{
		Actor:    kernel.NewUserActor(claims.UserID),
		TenantID: claims.TenantID,
		Email:    claims.Email,
		Name:     claims.Name,
		Scopes:   effective,
	}

	c.Locals("auth", authContext)
	return c.Next()
}

// RequireScope - Requires a specific scope (works for both JWT and API keys)
func (am *UnifiedAuthMiddleware) RequireScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authContext, ok := GetAuthContext(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if !authContext.HasScope(scope) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":          "Insufficient permissions",
				"required_scope": scope,
			})
		}

		return c.Next()
	}
}

// RequireAnyScope - Requires any of the provided scopes (works for both JWT and API keys)
func (am *UnifiedAuthMiddleware) RequireAnyScope(scopes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authContext, ok := GetAuthContext(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if !authContext.HasAnyScope(scopes...) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":           "Insufficient permissions",
				"required_scopes": scopes,
			})
		}

		return c.Next()
	}
}

// RequireAllScopes - Requires ALL specified scopes (AND logic, works for both JWT and API keys)
func (am *UnifiedAuthMiddleware) RequireAllScopes(scopes ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authContext, ok := GetAuthContext(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if !authContext.HasAllScopes(scopes...) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":           "Insufficient permissions",
				"required_scopes": scopes,
			})
		}

		return c.Next()
	}
}

// Helper functions
func extractAPIKey(c *fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && (parts[0] == "Bearer" || parts[0] == "X-API-Key") {
			if apikey.ValidateAPIKeyFormat(parts[1]) {
				return parts[1]
			}
		}
	}

	apiKeyHeader := c.Get("X-API-Key")
	if apiKeyHeader != "" && apikey.ValidateAPIKeyFormat(apiKeyHeader) {
		return apiKeyHeader
	}

	apiKeyQuery := c.Query("api_key")
	if apiKeyQuery != "" && apikey.ValidateAPIKeyFormat(apiKeyQuery) {
		return apiKeyQuery
	}

	return ""
}

// GetAuthContext helper to extract auth context from Fiber
func GetAuthContext(c *fiber.Ctx) (*kernel.AuthContext, bool) {
	authContext, ok := c.Locals("auth").(*kernel.AuthContext)
	return authContext, ok && authContext != nil && authContext.IsValid()
}

// AuthenticateUserJWT rejects API-key identity on user session endpoints.
func (am *UnifiedAuthMiddleware) AuthenticateUserJWT() fiber.Handler { return am.authenticateJWT }

// RequireUserActor prevents service credentials from widening their own constraints.
func (am *UnifiedAuthMiddleware) RequireUserActor() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ac, ok := GetAuthContext(c)
		if !ok || !ac.Actor.IsUser() {
			return fiber.NewError(fiber.StatusForbidden, "User authentication required")
		}
		return c.Next()
	}
}
