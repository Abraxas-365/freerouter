package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/iam"
	"github.com/Abraxas-365/freerouter/internal/iam/tenant"
	"github.com/Abraxas-365/freerouter/internal/iam/user"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type testUsers struct {
	user.UserRepository
	member *user.User
}

func (r testUsers) FindByID(context.Context, kernel.UserID, kernel.TenantID) (*user.User, error) {
	return r.member, nil
}
func (r testUsers) FindByEmailAcrossTenants(context.Context, string) ([]*user.User, error) {
	return []*user.User{r.member}, nil
}

type testTenants struct {
	tenant.TenantRepository
	entity *tenant.Tenant
}

func (r testTenants) FindByID(context.Context, kernel.TenantID) (*tenant.Tenant, error) {
	return r.entity, nil
}

type testSessions struct {
	SessionRepository
	session *UserSession
}

func (r testSessions) FindSession(context.Context, string) (*UserSession, error) {
	return r.session, nil
}

type testScopes struct {
	scopes []string
	err    error
}

func (r testScopes) GetEffectiveScopes(context.Context, kernel.UserID, kernel.TenantID) ([]string, error) {
	return r.scopes, r.err
}

func TestJWTCurrentEligibilityAndAuthority(t *testing.T) {
	tokens := NewJWTServiceFromConfig(&config.JWTConfig{SecretKey: "test-secret-not-for-production", AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour})
	member := &user.User{ID: "user", TenantID: "tenant", Status: user.UserStatusActive, EmailVerified: true, OTPEnabled: true, CredentialVersion: 2}
	organization := &tenant.Tenant{ID: "tenant", Status: tenant.TenantStatusActive}
	session := &UserSession{ID: "session", UserID: "user", TenantID: "tenant", ExpiresAt: time.Now().Add(time.Hour)}
	for _, tc := range []struct {
		name          string
		version       int64
		sessionID     string
		scopes        []string
		resolutionErr error
		mutate        func()
		want          int
	}{
		{name: "current authority", version: 2, sessionID: "session", scopes: []string{"gateway:chat"}, want: 204},
		{name: "claim wildcard is not authority", version: 2, sessionID: "session", want: 403},
		{name: "scope resolution fails closed", version: 2, sessionID: "session", resolutionErr: errors.New("database unavailable"), want: 401},
		{name: "old credential after reinstatement", version: 1, sessionID: "session", scopes: []string{"*"}, want: 401},
		{name: "legacy unbound token", version: 2, scopes: []string{"*"}, want: 401},
		{name: "expired session", version: 2, sessionID: "session", scopes: []string{"*"}, mutate: func() { session.ExpiresAt = time.Now().Add(-time.Hour) }, want: 401},
		{name: "suspended user", version: 2, sessionID: "session", scopes: []string{"*"}, mutate: func() { member.Status = user.UserStatusSuspended }, want: 401},
		{name: "suspended tenant", version: 2, sessionID: "session", scopes: []string{"*"}, mutate: func() { organization.Status = tenant.TenantStatusSuspended }, want: 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			member.Status = user.UserStatusActive
			organization.Status = tenant.TenantStatusActive
			session.ExpiresAt = time.Now().Add(time.Hour)
			if tc.mutate != nil {
				tc.mutate()
			}
			token, err := tokens.GenerateAccessToken("user", "tenant", map[string]any{"credential_version": tc.version, "session_id": tc.sessionID, "scopes": []string{"*"}})
			if err != nil {
				t.Fatal(err)
			}
			middleware := NewAPIKeyMiddleware(nil, tokens, testUsers{member: member}, testTenants{entity: organization}, "access_token", testSessions{session: session}, testScopes{scopes: tc.scopes, err: tc.resolutionErr})
			app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error { return c.SendStatus(401) }})
			app.Get("/", middleware.Authenticate(), middleware.RequireScope("gateway:chat"), func(c *fiber.Ctx) error { return c.SendStatus(204) })
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestReturningMicrosoftIdentityDoesNotRequireNewEmailAssertion(t *testing.T) {
	member := &user.User{ID: "user", TenantID: "tenant", Email: "member@example.com", Status: user.UserStatusActive, EmailVerified: true, OAuthProvider: iam.OAuthProviderMicrosoft, OAuthProviderID: "subject"}
	h := &AuthHandlers{userRepo: testUsers{member: member}, tenantRepo: testTenants{entity: &tenant.Tenant{ID: "tenant", Status: tenant.TenantStatusActive}}}
	info := &OAuthUserInfo{ID: "subject", Email: member.Email, EmailVerified: false}
	if _, _, err := h.findOrCreateUser(context.Background(), info, iam.OAuthProviderMicrosoft, nil, ""); err != nil {
		t.Fatalf("returning linked identity rejected: %v", err)
	}
	if _, _, err := h.findOrCreateUser(context.Background(), info, iam.OAuthProviderMicrosoft, map[string]interface{}{"invitation_token": "token"}, ""); err == nil {
		t.Fatal("unverified invitation linking accepted")
	}
}

func TestTenantWildcardNeverBypassesOwnership(t *testing.T) {
	app := fiber.New()
	app.Get("/:tenantId", func(c *fiber.Ctx) error {
		c.Locals("auth", &kernel.AuthContext{Actor: kernel.NewUserActor("user"), TenantID: "own", Scopes: []string{"*"}})
		return c.Next()
	}, ValidateTenantAccess(), func(c *fiber.Ctx) error { return c.SendStatus(204) })
	for path, want := range map[string]int{"/own": 204, "/other": 403} {
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("%s: got %d", path, response.StatusCode)
		}
	}
}
