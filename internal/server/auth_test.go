package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/iamkit/sdk/authclient"
	"github.com/gofiber/fiber/v2"
)

// captureAuthorization runs normalizeAPIKeyHeader against a fiber.Ctx built
// from the given request headers and returns the resulting Authorization
// header value.
func captureAuthorization(t *testing.T, setHeaders func(req *http.Request)) string {
	t.Helper()

	app := fiber.New()
	var got string
	app.Get("/probe", func(c *fiber.Ctx) error {
		normalizeAPIKeyHeader(c)
		got = c.Get("Authorization")
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/probe", nil)
	setHeaders(req)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return got
}

func TestNormalizeAPIKeyHeader_XAPIKeyOnly(t *testing.T) {
	got := captureAuthorization(t, func(req *http.Request) {
		req.Header.Set("X-Api-Key", "ik_svc_abc123")
	})
	want := "Bearer ik_svc_abc123"
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestNormalizeAPIKeyHeader_AuthorizationTakesPrecedence(t *testing.T) {
	got := captureAuthorization(t, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer original-token")
		req.Header.Set("X-Api-Key", "ik_svc_should_be_ignored")
	})
	want := "Bearer original-token"
	if got != want {
		t.Errorf("Authorization = %q, want %q (X-Api-Key must not override an existing Authorization header)", got, want)
	}
}

func TestNormalizeAPIKeyHeader_NoCredentials(t *testing.T) {
	got := captureAuthorization(t, func(req *http.Request) {})
	if got != "" {
		t.Errorf("Authorization = %q, want empty", got)
	}
}

func TestNormalizeAPIKeyHeader_BlankXAPIKeyIgnored(t *testing.T) {
	got := captureAuthorization(t, func(req *http.Request) {
		req.Header.Set("X-Api-Key", "   ")
	})
	if got != "" {
		t.Errorf("Authorization = %q, want empty (blank X-Api-Key should not set a header)", got)
	}
}

// ── Mock IAMKit server for ik_svc_ token exchange tests ──

// fakeIAMKit spins up a test HTTP server that simulates the two IAMKit
// endpoints used by AuthMiddleware: /identity/v1/machine-token and
// /identity/v1/introspect.
type fakeIAMKit struct {
	server             *httptest.Server
	machineTokenCalls  atomic.Int64
	introspectCalls    atomic.Int64

	// Configuration for the mock responses.
	machineTokenJWT string
	machineTokenTTL int
	claims          map[string]any
	env             string
	app             string
	resource        string
	audience        string
	issuer          string
}

func newFakeIAMKit() *fakeIAMKit {
	f := &fakeIAMKit{
		machineTokenJWT: "mock-jwt-from-exchange",
		machineTokenTTL: 300,
		env:             "env-1",
		app:             "app-1",
		resource:        "res-1",
		audience:        "https://freerouter.test",
		issuer:          "https://iamkit.test",
	}

	f.claims = map[string]any{
		"active": true,
		"claims": map[string]any{
			"sub":              "svc-account-123",
			"iss":              f.issuer,
			"aud":              []string{f.audience},
			"environment_id":   f.env,
			"application_id":   f.app,
			"resource_id":      f.resource,
			"purpose":          "machine",
			"permissions":      []string{"freerouter:gateway:invoke"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/identity/v1/machine-token", f.handleMachineToken)
	mux.HandleFunc("/identity/v1/introspect", f.handleIntrospect)
	f.server = httptest.NewServer(mux)
	return f
}

func (f *fakeIAMKit) handleMachineToken(w http.ResponseWriter, r *http.Request) {
	f.machineTokenCalls.Add(1)

	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ik_svc_") {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "UNAUTHORIZED", "message": "bad cred"}})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": f.machineTokenJWT,
		"token_type":   "Bearer",
		"expires_in":   f.machineTokenTTL,
	})
}

func (f *fakeIAMKit) handleIntrospect(w http.ResponseWriter, r *http.Request) {
	f.introspectCalls.Add(1)

	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+f.machineTokenJWT && auth != "Bearer valid-jwt" {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "UNAUTHORIZED", "message": "bad token"}})
		return
	}

	// Read body to properly consume it (IAMKit SDK sends environment_id, audience).
	_, _ = io.ReadAll(r.Body)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f.claims)
}

func (f *fakeIAMKit) close() {
	f.server.Close()
}

func (f *fakeIAMKit) config() config.IAMKit {
	return config.IAMKit{
		BaseURL:       f.server.URL,
		EnvironmentID: f.env,
		ApplicationID: f.app,
		ResourceID:    f.resource,
		JWTIssuer:     f.issuer,
		Audience:      f.audience,
	}
}

func TestAuthMiddleware_ServiceAccountTokenExchanged(t *testing.T) {
	fake := newFakeIAMKit()
	defer fake.close()

	auth := authclient.New(fake.server.URL)
	cfg := fake.config()

	app := fiber.New()
	app.Use(AuthMiddleware(auth, cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		claims := Claims(c)
		if claims == nil {
			return c.SendStatus(500)
		}
		return c.JSON(fiber.Map{"sub": claims.Subject})
	})

	// Send ik_svc_ token directly as Bearer.
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ik_svc_test-secret")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body: %s", resp.StatusCode, body)
	}

	if fake.machineTokenCalls.Load() != 1 {
		t.Errorf("MachineToken calls = %d, want 1", fake.machineTokenCalls.Load())
	}
	if fake.introspectCalls.Load() != 1 {
		t.Errorf("Introspect calls = %d, want 1", fake.introspectCalls.Load())
	}
}

func TestAuthMiddleware_ServiceAccountTokenCached(t *testing.T) {
	fake := newFakeIAMKit()
	defer fake.close()

	auth := authclient.New(fake.server.URL)
	cfg := fake.config()

	app := fiber.New()
	app.Use(AuthMiddleware(auth, cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	// First call — should exchange.
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("Authorization", "Bearer ik_svc_cached-secret")
	resp1, _ := app.Test(req1, -1)
	_ = resp1.Body.Close()
	if resp1.StatusCode != 204 {
		t.Fatalf("first call: status = %d, want 204", resp1.StatusCode)
	}

	// Second call — should use cached JWT, no new MachineToken call.
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer ik_svc_cached-secret")
	resp2, _ := app.Test(req2, -1)
	_ = resp2.Body.Close()
	if resp2.StatusCode != 204 {
		t.Fatalf("second call: status = %d, want 204", resp2.StatusCode)
	}

	if got := fake.machineTokenCalls.Load(); got != 1 {
		t.Errorf("MachineToken calls = %d, want 1 (second call should be cached)", got)
	}
	// Introspect is called both times (JWT validation is always online).
	if got := fake.introspectCalls.Load(); got != 2 {
		t.Errorf("Introspect calls = %d, want 2", got)
	}
}

func TestAuthMiddleware_RegularJWT_NoExchange(t *testing.T) {
	fake := newFakeIAMKit()
	defer fake.close()

	auth := authclient.New(fake.server.URL)
	cfg := fake.config()

	app := fiber.New()
	app.Use(AuthMiddleware(auth, cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	// Send a regular JWT — should skip MachineToken exchange entirely.
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := fake.machineTokenCalls.Load(); got != 0 {
		t.Errorf("MachineToken calls = %d, want 0 (regular JWT should skip exchange)", got)
	}
}

func TestAuthMiddleware_ServiceAccountViaXAPIKey(t *testing.T) {
	fake := newFakeIAMKit()
	defer fake.close()

	auth := authclient.New(fake.server.URL)
	cfg := fake.config()

	app := fiber.New()
	app.Use(AuthMiddleware(auth, cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	// Send ik_svc_ via X-Api-Key header (Anthropic SDK style).
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Api-Key", "ik_svc_anthropic-style")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := fake.machineTokenCalls.Load(); got != 1 {
		t.Errorf("MachineToken calls = %d, want 1", got)
	}
}

func TestAuthMiddleware_InvalidServiceAccount_Returns401(t *testing.T) {
	fake := newFakeIAMKit()
	defer fake.close()

	// Make the mock reject ik_svc_ tokens by requiring a specific secret.
	origHandler := fake.server.Config.Handler
	fake.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/identity/v1/machine-token" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "UNAUTHORIZED", "message": "bad"}}) //nolint:errcheck
			return
		}
		origHandler.ServeHTTP(w, r)
	})

	auth := authclient.New(fake.server.URL)
	cfg := fake.config()

	app := fiber.New()
	app.Use(AuthMiddleware(auth, cfg))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ik_svc_bad-credential")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
