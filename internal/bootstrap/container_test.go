package bootstrap

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/testutil"
	"github.com/gofiber/fiber/v2"
)

const testAdminToken = "bootstrap-test-admin-token-0123456789"

// localConfig is a config for mode with real Postgres/Redis and no IAMKit
// at all: every IAMKIT_* setting is empty.
func localConfig(t *testing.T, mode config.AuthMode) config.Config {
	t.Helper()
	_, dsn := testutil.PostgresDSN(t)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	rdb := testutil.RedisClient(t)

	cfg := config.Config{
		Server: config.Server{Port: "0", Host: "127.0.0.1", Env: "test"},
		Auth:   config.Auth{Mode: mode},
		Database: config.Database{Host: u.Hostname(), Port: u.Port(), User: u.User.Username(), Password: pass,
			Name: strings.TrimPrefix(u.Path, "/"), SSLMode: "disable"},
		Redis:      config.Redis{Addr: rdb.Options().Addr},
		Encryption: config.Encryption{Key: strings.Repeat("ab", 32)},
	}
	if mode == config.AuthToken {
		cfg.Auth.AdminToken = testAdminToken
	}
	return cfg
}

func boot(t *testing.T, cfg config.Config) *fiber.App {
	t.Helper()
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("bootstrap.New(%s) without IAMKit: %v", cfg.Auth.Mode, err)
	}
	t.Cleanup(c.Cleanup)
	if c.Access != nil {
		t.Fatal("access module wired without IAMKit")
	}
	return c.Server.App()
}

func call(t *testing.T, app *fiber.App, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Host = "localhost:3000"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestBoot_TokenModeWithoutIAMKit(t *testing.T) {
	app := boot(t, localConfig(t, config.AuthToken))
	admin := map[string]string{"Authorization": "Bearer " + testAdminToken}

	if s, b := call(t, app, "GET", "/api/v1/config", "", nil); s != 200 || !strings.Contains(b, `"auth":"token"`) || strings.Contains(b, "iamkit") {
		t.Fatalf("config = %d %s, want token mode without IAMKit details", s, b)
	}
	if s, _ := call(t, app, "GET", "/api/v1/providers", "", nil); s != 401 {
		t.Fatalf("anonymous admin API = %d, want 401", s)
	}
	if s, b := call(t, app, "GET", "/api/v1/providers", "", admin); s != 200 {
		t.Fatalf("admin token = %d %s, want 200", s, b)
	}
	if s, _ := call(t, app, "GET", "/api/v1/access/users", "", admin); s != 404 {
		t.Fatalf("/access without IAMKit = %d, want 404", s)
	}

	// The admin issues a gateway key; it reaches /v1 and nothing else.
	s, b := call(t, app, "POST", "/api/v1/service-accounts", `{"name":"ci","expires_in":"1h"}`, admin)
	if s != 201 && s != 200 {
		t.Fatalf("create key = %d %s", s, b)
	}
	var cred struct{ ID, Secret string }
	if err := json.Unmarshal([]byte(b), &cred); err != nil || !strings.HasPrefix(cred.Secret, server.LocalKeyPrefix) {
		t.Fatalf("credential %s (%v), want an fr_ secret", b, err)
	}
	key := map[string]string{"Authorization": "Bearer " + cred.Secret}
	if s, b := call(t, app, "GET", "/v1/models", "", key); s != 200 {
		t.Fatalf("gateway with key = %d %s, want 200", s, b)
	}
	if s, b := call(t, app, "GET", "/v1/models", "", map[string]string{"X-Api-Key": cred.Secret}); s != 200 {
		t.Fatalf("gateway with X-Api-Key = %d %s, want 200", s, b)
	}
	if s, _ := call(t, app, "GET", "/api/v1/providers", "", key); s != 403 {
		t.Fatalf("gateway key on admin API = %d, want 403", s)
	}
	if s, b := call(t, app, "GET", "/api/v1/service-accounts", "", admin); s != 200 || !strings.Contains(b, cred.ID) {
		t.Fatalf("list keys = %d %s", s, b)
	}

	// A key that may manage keys can't mint one outliving itself.
	s, b = call(t, app, "POST", "/api/v1/service-accounts",
		`{"name":"key-admin","expires_in":"2h","permissions":["freerouter:service-accounts:write","freerouter:gateway:invoke"]}`, admin)
	if s != 201 {
		t.Fatalf("create key-admin = %d %s", s, b)
	}
	var keyAdmin struct{ Secret string }
	_ = json.Unmarshal([]byte(b), &keyAdmin)
	ka := map[string]string{"Authorization": "Bearer " + keyAdmin.Secret}
	for _, exp := range []string{"never", "3h", ""} {
		if s, b := call(t, app, "POST", "/api/v1/service-accounts", `{"name":"child","expires_in":"`+exp+`"}`, ka); s != 403 {
			t.Fatalf("key-admin minting expires_in=%q = %d %s, want 403", exp, s, b)
		}
	}
	if s, b := call(t, app, "POST", "/api/v1/service-accounts", `{"name":"child","expires_in":"1h"}`, ka); s != 201 {
		t.Fatalf("key-admin minting a shorter key = %d %s, want 201", s, b)
	}
	if s, _ := call(t, app, "DELETE", "/api/v1/service-accounts/"+cred.ID, "", admin); s/100 != 2 {
		t.Fatalf("revoke = %d", s)
	}
	if s, _ := call(t, app, "GET", "/v1/models", "", key); s != 401 {
		t.Fatalf("revoked key = %d, want 401", s)
	}

	// Console: trade the admin token for a session cookie.
	req := httptest.NewRequest("POST", "/api/v1/session", strings.NewReader(`{"token":"`+testAdminToken+`"}`))
	req.Host = "localhost:3000"
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("console login = %v %v", resp, err)
	}
	_ = resp.Body.Close()
	var cookie string
	for _, c := range resp.Cookies() {
		if c.Name == server.SessionCookie {
			cookie = c.Name + "=" + c.Value
		}
	}
	if s, b := call(t, app, "GET", "/api/v1/providers", "", map[string]string{"Cookie": cookie}); s != 200 {
		t.Fatalf("admin API with session cookie = %d %s", s, b)
	}
}

func TestBoot_NoneModeWithoutIAMKit(t *testing.T) {
	app := boot(t, localConfig(t, config.AuthNone))

	if s, b := call(t, app, "GET", "/api/v1/config", "", nil); s != 200 || !strings.Contains(b, `"auth":"none"`) {
		t.Fatalf("config = %d %s", s, b)
	}
	if s, b := call(t, app, "GET", "/api/v1/session", "", nil); s != 200 || !strings.Contains(b, `"authenticated":true`) {
		t.Fatalf("session = %d %s, want the local owner", s, b)
	}
	if s, b := call(t, app, "GET", "/api/v1/providers", "", nil); s != 200 {
		t.Fatalf("local admin API = %d %s, want 200", s, b)
	}
	if s, b := call(t, app, "GET", "/v1/models", "", map[string]string{"Authorization": "Bearer sk-whatever"}); s != 200 {
		t.Fatalf("local gateway = %d %s, want 200", s, b)
	}
	if s, _ := call(t, app, "GET", "/api/v1/providers", "", map[string]string{"X-Forwarded-For": "203.0.113.9"}); s != 403 {
		t.Fatalf("proxied request = %d, want 403", s)
	}
}

func TestBoot_NoneModeRefusesPublicListen(t *testing.T) {
	cfg := config.Config{Server: config.Server{Port: "0", Host: "0.0.0.0"}, Auth: config.Auth{Mode: config.AuthNone}}
	if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("New(none on 0.0.0.0) err = %v, want loopback refusal before touching infrastructure", err)
	}
}
