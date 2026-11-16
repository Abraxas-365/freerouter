package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/gofiber/fiber/v2"
)

// fakeKeys resolves one gateway key.
type fakeKeys struct{}

const goodKey = LocalKeyPrefix + "good"

func (fakeKeys) ResolveKey(_ context.Context, secret string) (Principal, error) {
	if secret == goodKey {
		return Principal{Subject: "key-1", Permissions: []string{PermGatewayInvoke}}, nil
	}
	return Principal{}, errx.Unauthorized("invalid or expired token")
}

// authApp mounts a's middleware, session routes, an endpoint returning the
// principal, and one guarded by providers:write.
func authApp(a Authenticator) *fiber.App {
	app := fiber.New()
	if s := a.Session(); s.Current != nil {
		app.Get("/api/v1/session", s.Current)
		app.Post("/api/v1/session", s.Login)
		app.Delete("/api/v1/session", s.Logout)
	}
	api := app.Group("/api/v1", a.Authenticate())
	api.All("/whoami", func(c *fiber.Ctx) error {
		p, _ := Caller(c)
		// Explicit 200: a middleware that wrote a refusal but still called
		// Next must not pass as a refusal.
		return c.Status(fiber.StatusOK).JSON(p)
	})
	api.Post("/providers", RequirePermissions(PermProvidersWrite), func(c *fiber.Ctx) error {
		if _, ok := Caller(c); !ok {
			return c.Status(fiber.StatusTeapot).SendString("reached without a principal")
		}
		return c.SendStatus(fiber.StatusCreated)
	})
	return app
}

type result struct {
	status int
	body   string
	header http.Header
}

func do(t *testing.T, app *fiber.App, method, path string, body string, headers map[string]string) result {
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
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return result{status: resp.StatusCode, body: string(b), header: resp.Header}
}

func principalOf(t *testing.T, r result) Principal {
	t.Helper()
	if r.status != 200 {
		t.Fatalf("status = %d, want 200; body %s", r.status, r.body)
	}
	var p Principal
	if err := json.Unmarshal([]byte(r.body), &p); err != nil {
		t.Fatalf("decode principal: %v (%s)", err, r.body)
	}
	return p
}

// ── Open (none mode) ────────────────────────────────────────────────

func TestOpen_LocalCallerIsOwner(t *testing.T) {
	app := authApp(Open{Keys: fakeKeys{}})
	for _, host := range []string{"localhost:3000", "127.0.0.1:3000", "[::1]:3000", "localhost"} {
		p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Host": host}))
		if p.Subject != LocalSubject || !p.Can(PermProvidersWrite) {
			t.Fatalf("host %s: principal %+v, want local owner", host, p)
		}
	}
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", nil)); p.Can(PermUsersWrite) {
		t.Fatal("local owner holds users:write; IAMKit user admin has no meaning without IAMKit")
	}
	if r := do(t, app, "POST", "/api/v1/providers", "", nil); r.status != 201 {
		t.Fatalf("owner write = %d, want 201", r.status)
	}
}

func TestOpen_RefusesNonLocalRequests(t *testing.T) {
	app := authApp(Open{})
	cases := map[string]map[string]string{
		"foreign Host (DNS rebinding)": {"Host": "evil.example:3000"},
		"X-Forwarded-For (proxy)":      {"X-Forwarded-For": "203.0.113.9"},
		"Forwarded (proxy)":            {"Forwarded": "for=203.0.113.9"},
		"Cf-Connecting-Ip (tunnel)":    {"Cf-Connecting-Ip": "203.0.113.9"},
		"cross-site browser request":   {"Sec-Fetch-Site": "cross-site"},
		"foreign Origin":               {"Origin": "https://evil.example"},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			if r := do(t, app, "GET", "/api/v1/whoami", "", h); r.status != 403 {
				t.Fatalf("status = %d, want 403; body %s", r.status, r.body)
			}
			if r := do(t, app, "GET", "/api/v1/session", "", h); r.status != 403 {
				t.Fatalf("session status = %d, want 403 (must not reveal the open API)", r.status)
			}
		})
	}
	// The console itself (same origin; Vite's proxy keeps the Host) is local.
	if r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Origin": "http://localhost:3000", "Sec-Fetch-Site": "same-origin"}); r.status != 200 {
		t.Fatalf("same origin = %d, want 200", r.status)
	}
}

// Another local app (any other loopback port) must not get the owner's
// access: same origin, or a listed loopback console origin, only.
func TestOpen_RefusesOtherLoopbackOrigins(t *testing.T) {
	app := authApp(Open{ConsoleOrigins: []string{"http://localhost:5173", "https://router.example.com"}})
	for _, h := range []map[string]string{
		{"Origin": "http://localhost:8888", "Sec-Fetch-Site": "same-site"},
		{"Origin": "http://127.0.0.1:3000"},
		{"Origin": "http://localhost:3000", "Sec-Fetch-Site": "same-site"},
		{"Sec-Fetch-Site": "same-site"},
		{"Origin": "null"},
		{"Origin": "https://router.example.com", "Sec-Fetch-Site": "cross-site"}, // listed, but not loopback
		{"Via": "1.1 proxy"},
		{"X-Forwarded-Server": "proxy"},
	} {
		if r := do(t, app, "POST", "/api/v1/providers", "", h); r.status != 403 {
			t.Fatalf("%v = %d, want 403", h, r.status)
		}
	}
	// The console on Vite's port calling the API directly (VITE_API_URL).
	if r := do(t, app, "POST", "/api/v1/providers", "", map[string]string{"Origin": "http://localhost:5173", "Sec-Fetch-Site": "same-site"}); r.status != 201 {
		t.Fatalf("listed console origin = %d, want 201", r.status)
	}
}

func TestOpen_GatewayKeys(t *testing.T) {
	app := authApp(Open{Keys: fakeKeys{}})
	// A key sent anyway is that key's principal, with its grants only.
	p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer " + goodKey}))
	if p.Subject != "key-1" || p.Can(PermProvidersWrite) {
		t.Fatalf("key principal %+v, want key-1 with gateway:invoke only", p)
	}
	if r := do(t, app, "POST", "/api/v1/providers", "", map[string]string{"X-Api-Key": goodKey}); r.status != 403 {
		t.Fatalf("key without providers:write = %d, want 403", r.status)
	}
	// A wrong local key is refused, never silently ignored.
	if r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer " + LocalKeyPrefix + "bad"}); r.status != 401 {
		t.Fatalf("bad key = %d, want 401", r.status)
	}
	// Any other bearer is ignored (SDKs that insist on an API key).
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer sk-anything"})); p.Subject != LocalSubject {
		t.Fatalf("foreign bearer principal %+v, want local", p)
	}
}

// ── Tokens (token mode) ─────────────────────────────────────────────

const adminToken = "0123456789abcdef0123456789abcdef"

func TestTokens_Bearer(t *testing.T) {
	app := authApp(Tokens{AdminToken: adminToken, Keys: fakeKeys{}})

	if r := do(t, app, "GET", "/api/v1/whoami", "", nil); r.status != 401 || !strings.Contains(r.body, `"UNAUTHORIZED"`) {
		t.Fatalf("no credentials = %d %s, want 401 UNAUTHORIZED", r.status, r.body)
	}
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer " + adminToken})); p.Subject != AdminSubject || !p.Can(PermProvidersWrite) {
		t.Fatalf("admin principal %+v", p)
	}
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"X-Api-Key": adminToken})); p.Subject != AdminSubject {
		t.Fatalf("X-Api-Key admin principal %+v", p)
	}
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer " + goodKey})); p.Subject != "key-1" {
		t.Fatalf("key principal %+v", p)
	}
	r := do(t, app, "POST", "/api/v1/providers", "", map[string]string{"Authorization": "Bearer " + goodKey})
	if r.status != 403 || !strings.Contains(r.body, "insufficient permissions") {
		t.Fatalf("key without providers:write = %d %s, want 403 insufficient permissions", r.status, r.body)
	}
	for _, bad := range []string{"Bearer wrong", "Bearer " + LocalKeyPrefix + "bad", "Basic " + adminToken, "Bearer " + adminToken + "x", "Bearer " + adminToken + " x"} {
		if r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": bad}); r.status != 401 {
			t.Fatalf("%q = %d, want 401", bad, r.status)
		}
	}
}

func sessionCookie(t *testing.T, r result) string {
	t.Helper()
	for _, c := range r.header.Values("Set-Cookie") {
		if strings.HasPrefix(c, SessionCookie+"=") {
			return strings.SplitN(c, ";", 2)[0]
		}
	}
	t.Fatalf("no %s cookie in %v", SessionCookie, r.header.Values("Set-Cookie"))
	return ""
}

func TestTokens_ConsoleSession(t *testing.T) {
	app := authApp(Tokens{AdminToken: adminToken})

	if r := do(t, app, "GET", "/api/v1/session", "", nil); r.status != 200 || !strings.Contains(r.body, `"authenticated":false`) {
		t.Fatalf("anonymous session = %d %s, want 200 authenticated:false", r.status, r.body)
	}
	if r := do(t, app, "POST", "/api/v1/session", `{"token":"wrong"}`, nil); r.status != 401 {
		t.Fatalf("login with wrong token = %d, want 401", r.status)
	}
	if r := do(t, app, "POST", "/api/v1/session", `{"token":"`+adminToken+`"}`, map[string]string{"Origin": "https://evil.example"}); r.status != 403 {
		t.Fatalf("cross-origin login = %d, want 403", r.status)
	}

	login := do(t, app, "POST", "/api/v1/session", `{"token":"`+adminToken+`"}`, map[string]string{"Origin": "http://localhost:3000"})
	if login.status != 200 {
		t.Fatalf("login = %d %s", login.status, login.body)
	}
	cookie := sessionCookie(t, login)
	if raw := strings.Join(login.header.Values("Set-Cookie"), ";"); !strings.Contains(raw, "HttpOnly") || !strings.Contains(raw, "SameSite=Strict") {
		t.Fatalf("session cookie flags: %s", raw)
	}
	if strings.Contains(cookie, adminToken) {
		t.Fatal("session cookie carries the admin token")
	}

	if r := do(t, app, "GET", "/api/v1/session", "", map[string]string{"Cookie": cookie}); !strings.Contains(r.body, `"authenticated":true`) {
		t.Fatalf("session after login: %s", r.body)
	}
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Cookie": cookie})); p.Subject != AdminSubject {
		t.Fatalf("cookie principal %+v", p)
	}
	if r := do(t, app, "POST", "/api/v1/providers", "", map[string]string{"Cookie": cookie, "Sec-Fetch-Site": "same-origin"}); r.status != 201 {
		t.Fatalf("same-origin write with cookie = %d, want 201", r.status)
	}
	// CSRF: the cookie rides along on a cross-site form post; refuse it.
	if r := do(t, app, "POST", "/api/v1/providers", "", map[string]string{"Cookie": cookie, "Sec-Fetch-Site": "cross-site"}); r.status != 403 {
		t.Fatalf("cross-site write with cookie = %d, want 403", r.status)
	}
	if r := do(t, app, "POST", "/api/v1/providers", "", map[string]string{"Cookie": cookie, "Origin": "https://evil.example"}); r.status != 403 {
		t.Fatalf("foreign-origin write with cookie = %d, want 403", r.status)
	}
	// A bearer wins over the cookie and is judged alone.
	if r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Cookie": cookie, "Authorization": "Bearer wrong"}); r.status != 401 {
		t.Fatalf("bad bearer with good cookie = %d, want 401", r.status)
	}

	// Rotating the admin token ends every session.
	rotated := authApp(Tokens{AdminToken: strings.Repeat("z", 32)})
	if r := do(t, rotated, "GET", "/api/v1/whoami", "", map[string]string{"Cookie": cookie}); r.status != 401 {
		t.Fatalf("cookie after token rotation = %d, want 401", r.status)
	}
	// Forged and tampered cookies are refused.
	for _, forged := range []string{SessionCookie + "=99999999999.deadbeef", SessionCookie + "=1.x", cookie + "0"} {
		if r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Cookie": forged}); r.status != 401 {
			t.Fatalf("forged cookie %q = %d, want 401", forged, r.status)
		}
	}

	logout := do(t, app, "DELETE", "/api/v1/session", "", map[string]string{"Cookie": cookie})
	if raw := strings.ToLower(strings.Join(logout.header.Values("Set-Cookie"), ";")); logout.status != 204 || !strings.Contains(raw, "expires=thu, 01 jan 1970") {
		t.Fatalf("logout = %d %s, want 204 expiring the cookie", logout.status, raw)
	}
}

// ── RequirePermissions ──────────────────────────────────────────────

func TestRequirePermissions_WithoutPrincipalIs401(t *testing.T) {
	app := fiber.New()
	app.Get("/x", RequirePermissions(PermUsageRead), func(c *fiber.Ctx) error { return c.SendStatus(200) })
	if r := do(t, app, "GET", "/x", "", nil); r.status != 401 {
		t.Fatalf("status = %d, want 401", r.status)
	}
}

func TestLocalPermissions(t *testing.T) {
	got := LocalPermissions()
	if len(got) != len(ValidPermissions)-4 {
		t.Fatalf("LocalPermissions = %d perms, want every one but users:* and roles:*", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("LocalPermissions not sorted: %v", got)
		}
	}
}

func TestTokens_ThrottlesAdminTokenGuesses(t *testing.T) {
	app := authApp(NewTokens(adminToken, fakeKeys{}))
	for i := 0; i < adminGuessLimit; i++ {
		if r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer wrong"}); r.status != 401 {
			t.Fatalf("guess %d = %d, want 401", i, r.status)
		}
	}
	r := do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer " + adminToken})
	if r.status != 429 || r.header.Get("Retry-After") == "" {
		t.Fatalf("after %d failures, even the right token = %d (Retry-After %q), want 429", adminGuessLimit, r.status, r.header.Get("Retry-After"))
	}
	if r := do(t, app, "POST", "/api/v1/session", `{"token":"`+adminToken+`"}`, nil); r.status != 429 {
		t.Fatalf("console login while throttled = %d, want 429", r.status)
	}
	// Gateway keys are unguessable and stay usable.
	if p := principalOf(t, do(t, app, "GET", "/api/v1/whoami", "", map[string]string{"Authorization": "Bearer " + goodKey})); p.Subject != "key-1" {
		t.Fatalf("key while throttled: %+v", p)
	}
}

// Behind a trusted proxy each client has its own bucket; from anyone else
// X-Forwarded-For is ignored, so it can't be rotated to dodge the limit.
func TestTokens_ThrottlePerClientBehindTrustedProxy(t *testing.T) {
	guess := func(app *fiber.App, xff string) int {
		req := httptest.NewRequest("GET", "/api/v1/whoami", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		req.Header.Set("X-Forwarded-For", xff)
		resp, err := app.Test(req, -1) // app.Test's peer is 0.0.0.0
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	build := func(proxies []string) *fiber.App {
		app := New("", WithTrustedProxies(proxies, "X-Forwarded-For")).App()
		app.Get("/api/v1/whoami", NewTokens(adminToken, nil).Authenticate(), func(c *fiber.Ctx) error { return c.SendStatus(200) })
		return app
	}

	trusted := build([]string{"0.0.0.0"})
	for i := 0; i < adminGuessLimit; i++ {
		guess(trusted, "203.0.113.1")
	}
	if s := guess(trusted, "203.0.113.1"); s != 429 {
		t.Fatalf("attacker behind proxy = %d, want 429", s)
	}
	if s := guess(trusted, "198.51.100.7"); s != 401 {
		t.Fatalf("another client behind the same proxy = %d, want 401 (not locked out)", s)
	}

	untrusted := build(nil)
	for i := 0; i < adminGuessLimit; i++ {
		guess(untrusted, fmt.Sprintf("203.0.113.%d", i))
	}
	if s := guess(untrusted, "203.0.113.200"); s != 429 {
		t.Fatalf("rotating X-Forwarded-For without a trusted proxy = %d, want 429", s)
	}
}

func TestFailureLimiter_Window(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newFailureLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	l.fail("a")
	l.fail("a")
	if _, blocked := l.blocked("a"); !blocked {
		t.Fatal("a not blocked after 2 failures")
	}
	if _, blocked := l.blocked("b"); blocked {
		t.Fatal("b blocked by a's failures")
	}
	now = now.Add(time.Minute)
	if _, blocked := l.blocked("a"); blocked {
		t.Fatal("a still blocked after the window")
	}
}

func TestTokens_SecureCookieBehindTLSProxy(t *testing.T) {
	app := authApp(Tokens{AdminToken: adminToken})
	r := do(t, app, "POST", "/api/v1/session", `{"token":"`+adminToken+`"}`,
		map[string]string{"Host": "router.example.com", "Origin": "https://router.example.com"})
	if r.status != 200 || !strings.Contains(strings.Join(r.header.Values("Set-Cookie"), ";"), "secure") {
		t.Fatalf("login from an https page = %d, cookie %v: want Secure", r.status, r.header.Values("Set-Cookie"))
	}
}
