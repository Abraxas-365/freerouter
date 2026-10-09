//go:build e2e

package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// WS-02 — authorization matrix (route × persona) and permission-change
// propagation. The route table below is cross-checked against the route
// registrations in the source (TestWS02_RouteTableMatchesSource) so a new
// route without a matrix row, or a row whose permission drifted from the
// code, fails the suite.

// ws02Missing is a syntactically valid UUID that never exists.
const ws02Missing = "0e2e0002-0000-4000-8000-000000000000"

// ws02Bad is a truncated JSON body: allowed callers get a 400 from the body
// parser before anything is created or mutated.
const ws02Bad = `{"ws02":`

type ws02Route struct {
	Method string
	Path   string // with :params already substituted
	Src    string // the registered pattern (for the source cross-check)
	Perm   string // required permission; "" = public
	Body   any
	// Destructive marks routes that mutate global state even with no body
	// (DELETE /usage/retention). The matrix snapshots and restores around them.
	Destructive bool
}

const (
	ws02PGwInvoke = "freerouter:gateway:invoke"
	ws02PGwWrite  = "freerouter:gateway:write"
	ws02PMetrics  = "freerouter:metrics:read"
	ws02PProvR    = "freerouter:providers:read"
	ws02PProvW    = "freerouter:providers:write"
	ws02PPKR      = "freerouter:provider-keys:read"
	ws02PPKW      = "freerouter:provider-keys:write"
	ws02PUsageR   = "freerouter:usage:read"
	ws02PUsageW   = "freerouter:usage:write"
	ws02PRLR      = "freerouter:rate-limits:read"
	ws02PRLW      = "freerouter:rate-limits:write"
	ws02PRoutR    = "freerouter:routing:read"
	ws02PRoutW    = "freerouter:routing:write"
	ws02PGRR      = "freerouter:guardrails:read"
	ws02PGRW      = "freerouter:guardrails:write"
	ws02PWHR      = "freerouter:webhooks:read"
	ws02PWHW      = "freerouter:webhooks:write"
	ws02PSAR      = "freerouter:service-accounts:read"
	ws02PSAW      = "freerouter:service-accounts:write"
	ws02PUsersR   = "freerouter:users:read"
	ws02PUsersW   = "freerouter:users:write"
	ws02PRolesR   = "freerouter:roles:read"
	ws02PRolesW   = "freerouter:roles:write"
)

func ws02Routes() []ws02Route {
	id := ws02Missing
	r := func(m, src, perm string, body any) ws02Route {
		p := strings.NewReplacer(":id", id, ":modelId", id).Replace(src)
		if body == nil && (m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch) {
			body = ws02Bad
		}
		return ws02Route{Method: m, Path: p, Src: src, Perm: perm, Body: body}
	}
	G, P, U, A, D := http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete
	rs := []ws02Route{
		r(G, "/health", "", nil),
		r(G, "/metrics", ws02PMetrics, nil),

		// providers / models / mappings / fallbacks
		r(P, "/api/v1/providers", ws02PProvW, nil),
		r(G, "/api/v1/providers", ws02PProvR, nil),
		r(G, "/api/v1/providers/:id", ws02PProvR, nil),
		r(U, "/api/v1/providers/:id", ws02PProvW, nil),
		r(D, "/api/v1/providers/:id", ws02PProvW, nil),
		r(P, "/api/v1/models", ws02PProvW, nil),
		r(G, "/api/v1/models", ws02PProvR, nil),
		r(G, "/api/v1/models/:id", ws02PProvR, nil),
		r(U, "/api/v1/models/:id", ws02PProvW, nil),
		r(D, "/api/v1/models/:id", ws02PProvW, nil),
		r(P, "/api/v1/mappings", ws02PProvW, nil),
		r(G, "/api/v1/mappings", ws02PProvR, nil),
		r(G, "/api/v1/mappings/:id", ws02PProvR, nil),
		r(U, "/api/v1/mappings/:id", ws02PProvW, nil),
		r(D, "/api/v1/mappings/:id", ws02PProvW, nil),
		r(P, "/api/v1/model-fallbacks", ws02PProvW, nil),
		r(G, "/api/v1/model-fallbacks/by-model/:modelId", ws02PProvR, nil),
		r(D, "/api/v1/model-fallbacks/:id", ws02PProvW, nil),

		// provider keys
		r(P, "/api/v1/provider-keys", ws02PPKW, nil),
		r(G, "/api/v1/provider-keys", ws02PPKR, nil),
		r(G, "/api/v1/provider-keys/:id", ws02PPKR, nil),
		r(U, "/api/v1/provider-keys/:id", ws02PPKW, nil),
		r(D, "/api/v1/provider-keys/:id", ws02PPKW, nil),

		// usage
		r(G, "/api/v1/usage", ws02PUsageR, nil),
		r(G, "/api/v1/usage/summary", ws02PUsageR, nil),
		r(G, "/api/v1/usage/retention", ws02PUsageR, nil),
		r(U, "/api/v1/usage/retention", ws02PUsageW, nil),
		{Method: D, Path: "/api/v1/usage/retention", Src: "/api/v1/usage/retention", Perm: ws02PUsageW, Destructive: true},
		r(G, "/api/v1/usage/:id", ws02PUsageR, nil),

		// rate limits
		r(P, "/api/v1/rate-limits", ws02PRLW, nil),
		r(G, "/api/v1/rate-limits", ws02PRLR, nil),
		r(G, "/api/v1/rate-limits/:id", ws02PRLR, nil),
		r(A, "/api/v1/rate-limits/:id", ws02PRLW, nil),
		r(D, "/api/v1/rate-limits/:id", ws02PRLW, nil),

		// routing configs
		r(P, "/api/v1/routing-configs", ws02PRoutW, nil),
		r(G, "/api/v1/routing-configs", ws02PRoutR, nil),
		r(G, "/api/v1/routing-configs/:id", ws02PRoutR, nil),
		r(A, "/api/v1/routing-configs/:id", ws02PRoutW, nil),
		r(D, "/api/v1/routing-configs/:id", ws02PRoutW, nil),

		// guardrails
		r(G, "/api/v1/guardrails/config", ws02PGRR, nil),
		r(U, "/api/v1/guardrails/config", ws02PGRW, nil),
		r(G, "/api/v1/guardrails/rules", ws02PGRR, nil),
		r(P, "/api/v1/guardrails/rules", ws02PGRW, nil),
		r(G, "/api/v1/guardrails/rules/:id", ws02PGRR, nil),
		r(A, "/api/v1/guardrails/rules/:id", ws02PGRW, nil),
		r(D, "/api/v1/guardrails/rules/:id", ws02PGRW, nil),
		r(G, "/api/v1/guardrails/violations", ws02PGRR, nil),

		// webhooks
		r(P, "/api/v1/webhooks", ws02PWHW, nil),
		r(G, "/api/v1/webhooks", ws02PWHR, nil),
		r(G, "/api/v1/webhooks/events", ws02PWHR, nil),
		r(G, "/api/v1/webhooks/:id", ws02PWHR, nil),
		r(A, "/api/v1/webhooks/:id", ws02PWHW, nil),
		r(D, "/api/v1/webhooks/:id", ws02PWHW, nil),
		r(G, "/api/v1/webhooks/:id/deliveries", ws02PWHR, nil),
		r(P, "/api/v1/webhooks/:id/test", ws02PWHW, nil),

		// gateway admin — scoped to a subject nobody uses so an allowed call
		// does not wipe other workstreams' cache entries.
		{Method: D, Path: "/api/v1/gateway/cache?subject=ws02-nobody", Src: "/api/v1/gateway/cache", Perm: ws02PGwWrite},

		// service accounts
		r(P, "/api/v1/service-accounts", ws02PSAW, nil),
		r(G, "/api/v1/service-accounts", ws02PSAR, nil),
		r(G, "/api/v1/service-accounts/applications", ws02PSAR, nil),
		r(D, "/api/v1/service-accounts/:id", ws02PSAW, nil),

		// access
		r(P, "/api/v1/access/users", ws02PUsersW, nil),
		r(G, "/api/v1/access/users", ws02PUsersR, nil),
		r(G, "/api/v1/access/users/:id", ws02PUsersR, nil),
		r(A, "/api/v1/access/users/:id", ws02PUsersW, nil),
		r(D, "/api/v1/access/users/:id", ws02PUsersW, nil),
		r(P, "/api/v1/access/roles", ws02PRolesW, nil),
		r(G, "/api/v1/access/roles", ws02PRolesR, nil),
		r(U, "/api/v1/access/roles/:id", ws02PRolesW, nil),
		r(D, "/api/v1/access/roles/:id", ws02PRolesW, nil),
		r(P, "/api/v1/access/role-assignments", ws02PRolesW, nil),
		r(G, "/api/v1/access/role-assignments", ws02PRolesR, nil),
		r(D, "/api/v1/access/role-assignments", ws02PRolesW, ws02Bad),

		// gateway (/v1) — every route needs gateway:invoke
		r(P, "/v1/chat/completions", ws02PGwInvoke, nil),
		r(P, "/v1/messages", ws02PGwInvoke, nil),
		r(P, "/v1/responses", ws02PGwInvoke, nil),
		r(G, "/v1/models", ws02PGwInvoke, nil),
		r(P, "/v1/cost/estimate", ws02PGwInvoke, nil),
		r(P, "/v1/embeddings", ws02PGwInvoke, nil),
		r(P, "/v1/audio/transcriptions", ws02PGwInvoke, nil),
		r(P, "/v1/audio/speech", ws02PGwInvoke, nil),
		r(P, "/v1/moderations", ws02PGwInvoke, nil),
		r(P, "/v1/rerank", ws02PGwInvoke, nil),
		r(P, "/v1/images/generations", ws02PGwInvoke, nil),
	}
	return rs
}

// ── personas ─────────────────────────────────────────────────────────

type ws02Persona struct {
	Name  string
	Token string          // "" = anonymous
	Perms map[string]bool // nil = unauthenticated (expect 401 everywhere protected)
}

func ws02SetOf(ps ...string) map[string]bool {
	m := map[string]bool{}
	for _, p := range ps {
		m[p] = true
	}
	return m
}

var (
	ws02PersonasOnce sync.Once
	ws02PersonasVal  []ws02Persona
)

// ws02Personas logs the user personas in once and returns every persona
// that can present a credential to FreeRouter.
func ws02Personas(t *testing.T) []ws02Persona {
	t.Helper()
	f := FX(t)
	ws02PersonasOnce.Do(func() {
		all := ws02SetOf(f.Permissions...)
		var reads []string
		for _, p := range f.Permissions {
			if strings.HasSuffix(p, ":read") {
				reads = append(reads, p)
			}
		}
		adminTok := ws02Login(t, f.Personas["admin"])
		viewerTok := ws02Login(t, f.Personas["viewer"])
		provTok := ws02Login(t, f.Personas["providers_only"])
		ws02PersonasVal = []ws02Persona{
			{Name: "anon", Token: "", Perms: nil},
			{Name: "expired_key", Token: f.Personas["expired_key"].Secret, Perms: nil},
			{Name: "gw_key", Token: f.Personas["gw_key"].Secret, Perms: ws02SetOf(ws02PGwInvoke)},
			{Name: "noperm_key", Token: f.Personas["noperm_key"].Secret, Perms: ws02SetOf(ws02PMetrics)},
			{Name: "admin_key", Token: f.Personas["admin_key"].Secret, Perms: all},
			{Name: "admin", Token: adminTok, Perms: all},
			{Name: "viewer", Token: viewerTok, Perms: ws02SetOf(reads...)},
			{Name: "providers_only", Token: provTok, Perms: ws02SetOf(ws02PProvR, ws02PProvW)},
		}
	})
	if ws02PersonasVal == nil {
		t.Fatal("persona setup failed")
	}
	return ws02PersonasVal
}

// expectation for one cell.
func ws02Expect(r ws02Route, p ws02Persona) string {
	switch {
	case r.Perm == "":
		return "allow"
	case p.Perms == nil:
		return "401"
	case p.Perms[r.Perm]:
		return "allow"
	default:
		return "403"
	}
}

func ws02CellOK(expect string, status int) bool {
	switch expect {
	case "401":
		return status == http.StatusUnauthorized
	case "403":
		return status == http.StatusForbidden
	default:
		return status != http.StatusUnauthorized && status != http.StatusForbidden
	}
}

type ws02Cell struct {
	Route, Persona, Expect string
	Status                 int
}

var (
	ws02MatrixMu sync.Mutex
	ws02Matrix   []ws02Cell
)

// TestWS02_Matrix drives every registered route as every persona.
func TestWS02_Matrix(t *testing.T) {
	f := FX(t)
	personas := ws02Personas(t)
	admin := AdminAPI(t)

	for _, rt := range ws02Routes() {
		rt := rt
		t.Run(rt.Method+" "+rt.Src, func(t *testing.T) {
			for _, p := range personas {
				p := p
				t.Run(p.Name, func(t *testing.T) {
					expect := ws02Expect(rt, p)
					var c *Client
					if p.Token == "" {
						c = Anon(f.URLs.Server)
					} else {
						c = Bearer(f.URLs.Server, p.Token)
					}
					if rt.Destructive && expect == "allow" {
						// Snapshot the global singleton and restore it right away.
						prev := admin.Get(t, "/usage/retention")
						t.Cleanup(func() {
							if prev.Status == 200 {
								if rr := admin.Put(t, "/usage/retention", prev.Body); rr.Status != 200 {
									t.Errorf("restore retention: %d %s", rr.Status, rr.Body)
								}
							}
						})
					}
					res := ws02Do(t, c, rt.Method, rt.Path, rt.Body, p.Perms != nil)
					ws02MatrixMu.Lock()
					ws02Matrix = append(ws02Matrix, ws02Cell{rt.Method + " " + rt.Src, p.Name, expect, res.Status})
					ws02MatrixMu.Unlock()
					if !ws02CellOK(expect, res.Status) {
						t.Errorf("%s %s as %s: want %s, got %d %s", rt.Method, rt.Path, p.Name, expect, res.Status, ws02Trunc(res.Body))
					}
				})
			}
		})
	}
	t.Cleanup(ws02WriteMatrix)
}

// TestWS02_AllowedCellsAreClientErrors: an allowed caller sending a harmless
// invalid body / unknown id must get 2xx/4xx, never a 5xx.
func TestWS02_AllowedCellsAreClientErrors(t *testing.T) {
	f := FX(t)
	c := Bearer(f.URLs.Server, f.Personas["admin_key"].Secret)
	for _, rt := range ws02Routes() {
		if rt.Destructive {
			continue // exercised (with restore) in the matrix
		}
		rt := rt
		t.Run(rt.Method+" "+rt.Src, func(t *testing.T) {
			res := ws02Do(t, c, rt.Method, rt.Path, rt.Body, true)
			if res.Status >= 500 {
				t.Errorf("%s %s: want 2xx/4xx for unknown id / invalid body, got %d %s", rt.Method, rt.Path, res.Status, ws02Trunc(res.Body))
			}
		})
	}
}

// TestWS02_UserLoginBoundaries covers the personas that cannot obtain a
// FreeRouter token at all.
func TestWS02_UserLoginBoundaries(t *testing.T) {
	f := FX(t)
	login := func(p Persona, org string) Resp { return ws02LoginRaw(t, org, p.Email, p.Password) }
	t.Run("suspended user cannot log in", func(t *testing.T) {
		if r := login(f.Personas["suspended"], f.IAMKit.OrganizationID); r.Status != 401 {
			t.Fatalf("want 401, got %d %s", r.Status, r.Body)
		}
	})
	t.Run("foreign user cannot log in to FreeRouter's org", func(t *testing.T) {
		if r := login(f.Personas["foreign"], f.IAMKit.OrganizationID); r.Status != 401 {
			t.Fatalf("want 401, got %d %s", r.Status, r.Body)
		}
	})
	t.Run("foreign user cannot obtain a FreeRouter token via its home org", func(t *testing.T) {
		if r := login(f.Personas["foreign"], f.Boundary["foreign_org_id"]); r.Status != 401 {
			t.Fatalf("want 401, got %d %s", r.Status, r.Body)
		}
	})
	t.Run("org member with no role cannot log in (IAMKit: no access to resource)", func(t *testing.T) {
		// Documented behaviour: IAMKit refuses login when the user has no
		// permission on the resource; the console shows the generic
		// "invalid credentials" message (see finding WS02-3).
		if r := login(f.Personas["noperm"], f.IAMKit.OrganizationID); r.Status != 401 {
			t.Fatalf("want 401, got %d %s", r.Status, r.Body)
		}
	})
	t.Run("correct password for viewer still works (control)", func(t *testing.T) {
		if r := login(f.Personas["viewer"], f.IAMKit.OrganizationID); r.Status != 200 {
			t.Fatalf("want 200, got %d %s", r.Status, r.Body)
		}
	})
}

// TestWS02_CredentialShapes: malformed / alternative credential transports.
func TestWS02_CredentialShapes(t *testing.T) {
	f := FX(t)
	do := func(path string, h map[string]string) Resp {
		c := &Client{Base: f.URLs.Server, Headers: h, HTTP: httpClient}
		return c.Get(t, path)
	}
	gw := f.Personas["gw_key"].Secret
	cases := []struct {
		name string
		path string
		h    map[string]string
		want int
	}{
		{"bearer garbage", "/v1/models", map[string]string{"Authorization": "Bearer not-a-token"}, 401},
		{"bearer fake ik_svc_", "/v1/models", map[string]string{"Authorization": "Bearer ik_svc_deadbeefdeadbeef"}, 401},
		{"basic scheme", "/v1/models", map[string]string{"Authorization": "Basic " + gw}, 401},
		{"empty bearer", "/v1/models", map[string]string{"Authorization": "Bearer "}, 401},
		{"raw secret no scheme", "/v1/models", map[string]string{"Authorization": gw}, 401},
		{"x-api-key gw on /v1", "/v1/models", map[string]string{"X-Api-Key": gw}, 200},
		{"x-api-key gw on /api/v1", "/api/v1/providers", map[string]string{"X-Api-Key": gw}, 403},
		{"x-api-key expired", "/v1/models", map[string]string{"X-Api-Key": f.Personas["expired_key"].Secret}, 401},
		{"bad bearer wins over good x-api-key", "/v1/models", map[string]string{"Authorization": "Bearer nope", "X-Api-Key": gw}, 401},
		{"tampered user JWT", "/api/v1/providers", map[string]string{"Authorization": "Bearer " + ws02Tamper(t)}, 401},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if r := do(tc.path, tc.h); r.Status != tc.want {
				t.Fatalf("want %d, got %d %s", tc.want, r.Status, ws02Trunc(r.Body))
			}
		})
	}
}

// ws02Tamper returns the viewer's JWT with its signature altered.
func ws02Tamper(t *testing.T) string {
	var tok string
	for _, p := range ws02Personas(t) {
		if p.Name == "viewer" {
			tok = p.Token
		}
	}
	parts := strings.Split(tok, ".")
	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	return parts[0] + "." + parts[1] + "." + string(sig)
}

// TestWS02_PermissionChange grants and revokes roles on a fresh user and
// records when existing tokens pick the change up.
func TestWS02_PermissionChange(t *testing.T) {
	f := FX(t)
	adminC := AdminAPI(t)
	admin := struct {
		Post   func(*testing.T, string, any) Resp
		Delete func(*testing.T, string) Resp
		Do     func(*testing.T, string, string, any) Resp
	}{
		Post:   func(t *testing.T, p string, b any) Resp { return ws02Do(t, adminC, http.MethodPost, p, b, true) },
		Delete: func(t *testing.T, p string) Resp { return ws02Do(t, adminC, http.MethodDelete, p, nil, true) },
		Do:     func(t *testing.T, m, p string, b any) Resp { return ws02Do(t, adminC, m, p, b, true) },
	}
	email := Uniq("ws02-perm") + "@e2e.test"
	pw := "ws02-password-12345"
	u := admin.Post(t, "/access/users", map[string]string{"email": email, "name": "WS02 Perm", "password": pw})
	if u.Status != 201 && u.Status != 200 {
		t.Fatalf("create user: %d %s", u.Status, u.Body)
	}
	var user struct{ ID string }
	u.JSON(t, &user)
	t.Cleanup(func() {
		admin.Delete(t, "/access/users/"+user.ID)
		Management(t).Delete(t, "/environments/"+f.IAMKit.EnvironmentID+"/users/"+user.ID+"/permanent")
	})
	mkRole := func(perms ...string) string {
		r := admin.Post(t, "/access/roles", map[string]any{"name": Uniq("ws02-role"), "permissions": perms})
		if r.Status != 201 && r.Status != 200 {
			t.Fatalf("create role: %d %s", r.Status, r.Body)
		}
		var role struct{ ID string }
		r.JSON(t, &role)
		t.Cleanup(func() { admin.Delete(t, "/access/roles/"+role.ID) })
		return role.ID
	}
	assign := func(role string) {
		r := admin.Post(t, "/access/role-assignments", map[string]string{"user_id": user.ID, "role_id": role})
		if r.Status >= 300 {
			t.Fatalf("assign: %d %s", r.Status, r.Body)
		}
		t.Cleanup(func() {
			admin.Do(t, http.MethodDelete, "/access/role-assignments", map[string]string{"user_id": user.ID, "role_id": role})
		})
	}
	unassign := func(role string) {
		r := admin.Do(t, http.MethodDelete, "/access/role-assignments", map[string]string{"user_id": user.ID, "role_id": role})
		if r.Status >= 300 {
			t.Fatalf("unassign: %d %s", r.Status, r.Body)
		}
	}
	loginRaw := func() Resp { return ws02LoginRaw(t, f.IAMKit.OrganizationID, email, pw) }
	tokens := func() (string, string) {
		r := loginRaw()
		if r.Status != 200 {
			t.Fatalf("login: %d %s", r.Status, r.Body)
		}
		var out struct{ AccessToken, RefreshToken string }
		m := r.Map(t)
		out.AccessToken, _ = m["access_token"].(string)
		out.RefreshToken, _ = m["refresh_token"].(string)
		return out.AccessToken, out.RefreshToken
	}
	status := func(tok, path string) int {
		return ws02Do(t, Bearer(f.URLs.API, tok), http.MethodGet, path, nil, false).Status
	}

	// 1. No role → cannot log in at all.
	if r := loginRaw(); r.Status != 401 {
		t.Fatalf("login with no role: want 401, got %d %s", r.Status, r.Body)
	}

	// 2. Grant providers:read → login works; providers allowed, provider-keys forbidden.
	roleProv := mkRole(ws02PProvR)
	assign(roleProv)
	tok1, ref1 := tokens()
	if s := status(tok1, "/providers"); s != 200 {
		t.Fatalf("providers after grant: want 200, got %d", s)
	}
	if s := status(tok1, "/provider-keys"); s != 403 {
		t.Fatalf("provider-keys without perm: want 403, got %d", s)
	}
	if s := ws02Do(t, Bearer(f.URLs.API, tok1), http.MethodPost, "/providers", ws02Bad, false).Status; s != 403 {
		t.Fatalf("read-only role on write route: want 403, got %d", s)
	}

	// 3. Grant provider-keys:read via a second role.
	rolePK := mkRole(ws02PPKR)
	assign(rolePK)
	t.Run("existing token does not gain new permission", func(t *testing.T) {
		// Permissions are embedded in the JWT; an existing token keeps the
		// old set until re-login/refresh. Documented, not a bug.
		if s := status(tok1, "/provider-keys"); s != 403 {
			t.Errorf("old token on newly granted route: got %d (documenting: expected 403)", s)
		}
	})
	t.Run("refresh picks up new permission", func(t *testing.T) {
		r := Anon(f.URLs.IAMKit).Post(t, "/identity/v1/refresh", map[string]string{
			"environment_id": f.IAMKit.EnvironmentID, "organization_id": f.IAMKit.OrganizationID,
			"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
			"refresh_token": ref1,
		})
		if r.Status != 200 {
			t.Fatalf("refresh: %d %s", r.Status, r.Body)
		}
		tok, _ := r.Map(t)["access_token"].(string)
		if s := status(tok, "/provider-keys"); s != 200 {
			t.Errorf("refreshed token on newly granted route: want 200, got %d", s)
		}
	})
	tok2, _ := tokens()
	if s := status(tok2, "/provider-keys"); s != 200 {
		t.Fatalf("re-login after grant: want 200, got %d", s)
	}

	// 4. Revoke providers:read → existing token must lose access immediately.
	unassign(roleProv)
	t.Run("revocation applies to existing token", func(t *testing.T) {
		s := status(tok2, "/providers")
		if s != 401 && s != 403 {
			t.Errorf("old token after revoke: want 401/403, got %d", s)
		}
	})
	tok3, _ := tokens()
	if s := status(tok3, "/providers"); s != 403 {
		t.Fatalf("re-login after revoke: providers want 403, got %d", s)
	}
	if s := status(tok3, "/provider-keys"); s != 200 {
		t.Fatalf("re-login after revoke: provider-keys want 200, got %d", s)
	}
}

// ── source cross-check ───────────────────────────────────────────────

// TestWS02_RouteTableMatchesSource parses the route registrations and
// permission guards in the source and diffs them against ws02Routes.
func TestWS02_RouteTableMatchesSource(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}

	// Permission constants → strings.
	consts := map[string]string{}
	for _, m := range regexp.MustCompile(`(Perm\w+)\s*=\s*"([^"]+)"`).FindAllStringSubmatch(read("internal/server/permissions.go"), -1) {
		consts[m[1]] = m[2]
	}

	// Mount points per handler Register* function (from internal/bootstrap/container.go).
	cont := read("internal/bootstrap/container.go")
	mounts := map[string]string{} // "pkg.Func" -> prefix
	for _, m := range regexp.MustCompile(`c\.(\w+)\.HTTP\.(Register\w*)\((api|v1)(?:\.Group\("([^"]*)"\))?\)`).FindAllStringSubmatch(cont, -1) {
		base := "/api/v1"
		if m[3] == "v1" {
			base = "/v1"
		}
		mounts[m[1]+"."+m[2]] = base + m[4]
	}
	modDir := map[string]string{
		"Provider": "provider/adapters/providerhttp", "ProviderKey": "providerkey/adapters/providerkeyhttp",
		"Usage": "usage/adapters/usagehttp", "RateLimit": "ratelimit/adapters/ratelimithttp",
		"RoutingConfig": "routingconfig/adapters/routingconfighttp", "Guardrail": "guardrail/adapters/guardrailhttp",
		"Webhook": "webhook/adapters/webhookhttp", "Gateway": "gateway/adapters/gatewayhttp",
		"APIKey": "apikey/adapters/apikeyhttp", "Access": "access/adapters/accesshttp",
	}

	src := map[string]string{} // "METHOD /path" -> perm
	src["GET /health"] = ""
	if !strings.Contains(cont, `app.Get("/metrics"`) || !strings.Contains(cont, "server.RequirePermissions(server.PermMetricsRead)") {
		t.Errorf("container.go: /metrics registration changed")
	}
	src["GET /metrics"] = consts["PermMetricsRead"]
	gwV1Guard := regexp.MustCompile(`app\.Group\("/v1",\s*authenticate,\s*server\.RequirePermissions\(server\.(\w+)\)`).FindStringSubmatch(cont)
	if gwV1Guard == nil {
		t.Fatal("container.go: /v1 group guard not found")
	}

	funcRe := regexp.MustCompile(`^func \(h \*?\w+\) (Register\w*)\(r fiber\.Router\)`)
	groupRe := regexp.MustCompile(`^\s*(\w+) := r\.Group\("([^"]*)"\)`)
	routeRe := regexp.MustCompile(`^\s*(\w+)\.(Get|Post|Put|Patch|Delete)\("([^"]*)"(?:,\s*server\.RequirePermissions\(server\.(\w+)\))?`)
	for mod, dir := range modDir {
		lines := strings.Split(read("internal/"+dir+"/handler.go"), "\n")
		fn := ""
		groups := map[string]string{}
		for _, ln := range lines {
			if m := funcRe.FindStringSubmatch(ln); m != nil {
				fn, groups = m[1], map[string]string{"r": ""}
				continue
			}
			if strings.HasPrefix(ln, "}") {
				fn = ""
				continue
			}
			if fn == "" {
				continue
			}
			if m := groupRe.FindStringSubmatch(ln); m != nil {
				groups[m[1]] = m[2]
				continue
			}
			if m := routeRe.FindStringSubmatch(ln); m != nil {
				g, ok := groups[m[1]]
				if !ok {
					continue
				}
				base, ok := mounts[mod+"."+fn]
				if !ok {
					t.Errorf("%s.%s not mounted in container.go", mod, fn)
					continue
				}
				p := strings.TrimSuffix(base+g+m[3], "/")
				perm := consts[m[4]]
				if strings.HasPrefix(base, "/v1") {
					perm = consts[gwV1Guard[1]]
				}
				src[strings.ToUpper(m[2])+" "+p] = perm
			}
		}
	}

	table := map[string]string{}
	for _, r := range ws02Routes() {
		table[r.Method+" "+r.Src] = r.Perm
	}
	var missing, extra, drift []string
	for k, v := range src {
		tv, ok := table[k]
		switch {
		case !ok:
			missing = append(missing, k)
		case tv != v:
			drift = append(drift, fmt.Sprintf("%s: table %q, source %q", k, tv, v))
		}
	}
	for k := range table {
		if _, ok := src[k]; !ok {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(drift)
	if len(src) < 80 {
		t.Errorf("parsed only %d routes from source; parser likely broken", len(src))
	}
	for _, m := range missing {
		t.Errorf("route registered but not in matrix: %s", m)
	}
	for _, m := range extra {
		t.Errorf("matrix route not registered in source: %s", m)
	}
	for _, m := range drift {
		t.Errorf("permission drift: %s", m)
	}
	t.Logf("source routes: %d, matrix routes: %d", len(src), len(table))
}

// ── helpers ──────────────────────────────────────────────────────────

// ws02Do performs a request, retrying while IAMKit throttles FreeRouter's
// backend (shared 120/min /api/v1 and 30/min /machine-token per-IP limits —
// see finding WS02-1). Those surface as 502 "... in IAMKit" or as 401
// "invalid service account credential" for a perfectly valid key.
func ws02Do(t *testing.T, c *Client, method, path string, body any, credentialed bool) Resp {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		r := c.Do(t, method, path, body)
		throttled := (r.Status == http.StatusBadGateway && strings.Contains(string(r.Body), "in IAMKit")) ||
			(credentialed && r.Status == http.StatusUnauthorized && strings.Contains(string(r.Body), "invalid service account credential"))
		if !throttled || time.Now().After(deadline) {
			return r
		}
		t.Logf("IAMKit throttling (%d %s); retrying", r.Status, ws02Trunc(r.Body))
		time.Sleep(3 * time.Second)
	}
}

// ws02LoginRaw posts to IAMKit /identity/v1/login, waiting out the shared
// 30/min per-IP login limiter (other workstreams log in concurrently).
func ws02LoginRaw(t *testing.T, org, email, password string) Resp {
	t.Helper()
	f := FX(t)
	deadline := time.Now().Add(3 * time.Minute)
	for {
		r := Anon(f.URLs.IAMKit).Post(t, "/identity/v1/login", map[string]string{
			"environment_id": f.IAMKit.EnvironmentID, "organization_id": org,
			"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
			"email": email, "password": password,
		})
		if r.Status != http.StatusTooManyRequests || time.Now().After(deadline) {
			return r
		}
		time.Sleep(5 * time.Second)
	}
}

// ws02Login returns an access token for a user persona (fails on non-200).
func ws02Login(t *testing.T, p Persona) string {
	t.Helper()
	r := ws02LoginRaw(t, FX(t).IAMKit.OrganizationID, p.Email, p.Password)
	if r.Status != 200 {
		t.Fatalf("login %s: %d %s", p.Email, r.Status, r.Body)
	}
	tok, _ := r.Map(t)["access_token"].(string)
	return tok
}

func ws02Trunc(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// ws02WriteMatrix dumps the observed matrix as markdown when
// WS02_MATRIX_OUT is set (used to build the findings report).
func ws02WriteMatrix() {
	out := os.Getenv("WS02_MATRIX_OUT")
	if out == "" {
		return
	}
	ws02MatrixMu.Lock()
	defer ws02MatrixMu.Unlock()
	var personas []string
	seen := map[string]bool{}
	cells := map[string]map[string]ws02Cell{}
	var routes []string
	for _, c := range ws02Matrix {
		if !seen[c.Persona] {
			seen[c.Persona] = true
			personas = append(personas, c.Persona)
		}
		if cells[c.Route] == nil {
			cells[c.Route] = map[string]ws02Cell{}
			routes = append(routes, c.Route)
		}
		cells[c.Route][c.Persona] = c
	}
	var sb strings.Builder
	sb.WriteString("| Route | " + strings.Join(personas, " | ") + " |\n|---|")
	for range personas {
		sb.WriteString("---|")
	}
	sb.WriteString("\n")
	for _, r := range routes {
		sb.WriteString("| `" + r + "` |")
		for _, p := range personas {
			c := cells[r][p]
			mark := ""
			if !ws02CellOK(c.Expect, c.Status) {
				mark = " ❌"
			}
			sb.WriteString(fmt.Sprintf(" %d%s |", c.Status, mark))
		}
		sb.WriteString("\n")
	}
	_ = os.WriteFile(out, []byte(sb.String()), 0o644)
}
