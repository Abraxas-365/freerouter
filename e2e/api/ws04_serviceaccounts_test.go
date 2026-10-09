//go:build e2e

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// WS-04: service accounts / API keys (/api/v1/service-accounts).

type ws04Cred struct {
	ID        string    `json:"id"`
	Secret    string    `json:"secret"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ws04Account struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ApplicationID string   `json:"application_id"`
	ResourceID    string   `json:"resource_id"`
	Permissions   []string `json:"permissions"`
	ExpiresIn     *string  `json:"expires_in"`
	ExpiresAt     *string  `json:"expires_at"`
	RevokedAt     *string  `json:"revoked_at"`
}

// ws04C wraps a Client and retries calls that FreeRouter failed with
// 502 "... in IAMKit": IAMKit throttles /api/v1 at 120 req/min per IP, a
// budget shared by every concurrent e2e worker (see FINDING WS04-4).
type ws04C struct{ c *Client }

func rl(c *Client) ws04C { return ws04C{c} }

func (w ws04C) Do(t *testing.T, method, path string, body any) Resp {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		r := w.c.Do(t, method, path, body)
		if r.Status != http.StatusBadGateway || !strings.Contains(string(r.Body), "in IAMKit") || time.Now().After(deadline) {
			return r
		}
		time.Sleep(3 * time.Second)
	}
}
func (w ws04C) Get(t *testing.T, path string) Resp { t.Helper(); return w.Do(t, "GET", path, nil) }
func (w ws04C) Post(t *testing.T, path string, b any) Resp {
	t.Helper()
	return w.Do(t, "POST", path, b)
}
func (w ws04C) Delete(t *testing.T, path string) Resp {
	t.Helper()
	return w.Do(t, "DELETE", path, nil)
}

func ws04Admin(t *testing.T) ws04C { return rl(AdminAPI(t)) }

// ws04Create creates a service account and registers its revocation.
func ws04Create(t *testing.T, body map[string]any) ws04Cred {
	t.Helper()
	r := ws04Admin(t).Post(t, "/service-accounts", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create %v: want 201, got %d %s", body, r.Status, r.Body)
	}
	var c ws04Cred
	r.JSON(t, &c)
	t.Cleanup(func() { ws04Admin(t).Delete(t, "/service-accounts/"+c.ID) })
	return c
}

func ws04List(t *testing.T) []ws04Account {
	t.Helper()
	r := ws04Admin(t).Get(t, "/service-accounts")
	if r.Status != 200 {
		t.Fatalf("list: %d %s", r.Status, r.Body)
	}
	var out []ws04Account
	r.JSON(t, &out)
	return out
}

func ws04Find(list []ws04Account, id string) (ws04Account, bool) {
	for _, a := range list {
		if a.ID == id {
			return a, true
		}
	}
	return ws04Account{}, false
}

// ws04ErrCode extracts the error code from either error envelope the server uses.
func ws04ErrCode(t *testing.T, r Resp) (code, msg string) {
	t.Helper()
	var flat struct{ Code, Message string }
	var nested struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	r.JSON(t, &flat)
	if flat.Code != "" {
		return flat.Code, flat.Message
	}
	r.JSON(t, &nested)
	return nested.Error.Code, nested.Error.Message
}

func ws04Expect(t *testing.T, r Resp, status int, code, msgSub string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("want %d, got %d %s", status, r.Status, r.Body)
	}
	c, m := ws04ErrCode(t, r)
	if code != "" && c != code {
		t.Fatalf("want code %s, got %s (%s)", code, c, r.Body)
	}
	if msgSub != "" && !strings.Contains(m, msgSub) {
		t.Fatalf("want message containing %q, got %q", msgSub, m)
	}
}

// TestWS04_CreateDefaultsAndListing: minimal create → gateway:invoke, ~24h default expiry,
// listed with FreeRouter's application/resource, secret never listed.
func TestWS04_CreateDefaultsAndListing(t *testing.T) {
	f := FX(t)
	name := Uniq("ws04-default")
	before := time.Now()
	c := ws04Create(t, map[string]any{"name": name})
	if c.ID == "" || !strings.HasPrefix(c.Secret, "ik_svc_") {
		t.Fatalf("credential = %+v; want id and ik_svc_ secret", c)
	}
	if d := c.ExpiresAt.Sub(before); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("default expiry %s from now; want ~24h", d)
	}
	r := ws04Admin(t).Get(t, "/service-accounts")
	if strings.Contains(string(r.Body), c.Secret) || strings.Contains(string(r.Body), `"secret"`) {
		t.Fatal("list response leaks secrets")
	}
	a, ok := ws04Find(ws04List(t), c.ID)
	if !ok {
		t.Fatalf("created account %s not listed", c.ID)
	}
	if a.Name != name || a.ApplicationID != f.IAMKit.ApplicationID || a.ResourceID != f.IAMKit.ResourceID {
		t.Fatalf("listed %+v; want name %s app %s res %s", a, name, f.IAMKit.ApplicationID, f.IAMKit.ResourceID)
	}
	if len(a.Permissions) != 1 || a.Permissions[0] != "freerouter:gateway:invoke" {
		t.Fatalf("default permissions = %v; want [freerouter:gateway:invoke]", a.Permissions)
	}
	// empty permissions array also defaults
	c2 := ws04Create(t, map[string]any{"name": Uniq("ws04-emptyperms"), "permissions": []string{}})
	a2, _ := ws04Find(ws04List(t), c2.ID)
	if len(a2.Permissions) != 1 || a2.Permissions[0] != "freerouter:gateway:invoke" {
		t.Fatalf("empty permissions → %v; want default gateway:invoke", a2.Permissions)
	}
}

// TestWS04_PermissionCombos: each key can do exactly what its permissions allow.
func TestWS04_PermissionCombos(t *testing.T) {
	f := FX(t)
	type probe struct {
		base, method, path string
		body               any
	}
	gwModels := probe{f.URLs.Gateway, "GET", "/models", nil}
	providers := probe{f.URLs.API, "GET", "/providers", nil}
	saList := probe{f.URLs.API, "GET", "/service-accounts", nil}
	usage := probe{f.URLs.API, "GET", "/usage", nil}
	webhooks := probe{f.URLs.API, "GET", "/webhooks", nil}

	cases := []struct {
		name  string
		perms []string
		allow []probe
		deny  []probe
	}{
		{"gateway", []string{"freerouter:gateway:invoke"}, []probe{gwModels}, []probe{providers, saList, usage}},
		{"providers-read", []string{"freerouter:providers:read"}, []probe{providers}, []probe{gwModels, saList, webhooks}},
		{"sa-read", []string{"freerouter:service-accounts:read"}, []probe{saList}, []probe{gwModels, providers}},
		{"gw+usage", []string{"freerouter:gateway:invoke", "freerouter:usage:read"}, []probe{gwModels, usage}, []probe{providers, saList}},
		{"all", f.Permissions, []probe{gwModels, providers, saList, usage, webhooks}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ws04Create(t, map[string]any{"name": Uniq("ws04-perm-" + tc.name), "permissions": tc.perms})
			for _, p := range tc.allow {
				if r := rl(Bearer(p.base, c.Secret)).Do(t, p.method, p.path, p.body); r.Status != 200 {
					t.Errorf("%s %s%s: want 200, got %d %s", p.method, p.base, p.path, r.Status, r.Body)
				}
			}
			for _, p := range tc.deny {
				r := rl(Bearer(p.base, c.Secret)).Do(t, p.method, p.path, p.body)
				if r.Status != http.StatusForbidden {
					t.Errorf("%s %s%s: want 403, got %d %s", p.method, p.base, p.path, r.Status, r.Body)
				}
			}
		})
	}

	t.Run("sa-read cannot create or revoke", func(t *testing.T) {
		c := ws04Create(t, map[string]any{"name": Uniq("ws04-saread"), "permissions": []string{"freerouter:service-accounts:read"}})
		cl := rl(Bearer(f.URLs.API, c.Secret))
		ws04Expect(t, cl.Post(t, "/service-accounts", map[string]any{"name": Uniq("ws04-nope")}), 403, "FORBIDDEN", "")
		ws04Expect(t, cl.Delete(t, "/service-accounts/"+f.Personas["gw_key"].ID), 403, "FORBIDDEN", "")
	})

	t.Run("new key works on gateway immediately incl. X-Api-Key and chat", func(t *testing.T) {
		c := ws04Create(t, map[string]any{"name": Uniq("ws04-immediate")})
		if r := Bearer(f.URLs.Gateway, c.Secret).Get(t, "/models"); r.Status != 200 {
			t.Fatalf("bearer /v1/models: %d %s", r.Status, r.Body)
		}
		x := &Client{Base: f.URLs.Gateway, Headers: map[string]string{"X-Api-Key": c.Secret}, HTTP: httpClient}
		if r := x.Get(t, "/models"); r.Status != 200 {
			t.Fatalf("x-api-key /v1/models: %d %s", r.Status, r.Body)
		}
		if r := Bearer(f.URLs.Gateway, c.Secret).Post(t, "/chat/completions", Chat("e2e-ok", "ws04 hi", false)); r.Status != 200 {
			t.Fatalf("chat: %d %s", r.Status, r.Body)
		}
	})
}

// TestWS04_CreateValidation: invalid input is rejected with 400 and nothing is created.
func TestWS04_CreateValidation(t *testing.T) {
	admin := ws04Admin(t)
	cases := []struct {
		name string
		body any
		msg  string
	}{
		{"empty name", map[string]any{"name": ""}, "name is required"},
		{"missing name", map[string]any{"permissions": []string{"freerouter:gateway:invoke"}}, "name is required"},
		{"whitespace name", map[string]any{"name": "   "}, "name is required"},
		{"unknown perm", map[string]any{"name": "ws04-bad", "permissions": []string{"freerouter:nope"}}, `unknown permission: "freerouter:nope"`},
		{"iam perm", map[string]any{"name": "ws04-bad", "permissions": []string{"iam:users:read"}}, `unknown permission: "iam:users:read"`},
		{"mixed valid+invalid", map[string]any{"name": "ws04-bad", "permissions": []string{"freerouter:gateway:invoke", "admin"}}, `unknown permission: "admin"`},
		{"duplicate perms", map[string]any{"name": "ws04-bad", "permissions": []string{"freerouter:gateway:invoke", "freerouter:gateway:invoke"}}, ""},
		{"application not a uuid", map[string]any{"name": "ws04-bad", "application_id": "nope"}, ""},
		{"application unknown", map[string]any{"name": "ws04-bad", "application_id": "00000000-0000-0000-0000-000000000000"}, ""},
		{"malformed json", "{", "invalid request body"},
		{"perms wrong type", `{"name":"ws04-bad","permissions":"freerouter:gateway:invoke"}`, "invalid request body"},
	}
	before := len(ws04List(t))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws04Expect(t, admin.Post(t, "/service-accounts", tc.body), 400, "VALIDATION", tc.msg)
		})
	}
	if after := len(ws04List(t)); after != before {
		// other workers may create accounts concurrently, so only check ours
		for _, a := range ws04List(t) {
			if a.Name == "ws04-bad" {
				t.Errorf("rejected create left account %+v", a)
			}
		}
	}

	t.Run("unauthenticated", func(t *testing.T) {
		ws04Expect(t, rl(Anon(FX(t).URLs.API)).Post(t, "/service-accounts", map[string]any{"name": "ws04-anon"}), 401, "UNAUTHORIZED", "")
		ws04Expect(t, rl(Anon(FX(t).URLs.API)).Get(t, "/service-accounts"), 401, "UNAUTHORIZED", "")
	})

	t.Run("application of IAM resource rejected", func(t *testing.T) {
		backendApp := ws04BackendAppID(t)
		r := admin.Post(t, "/service-accounts", map[string]any{"name": "ws04-bad", "application_id": backendApp})
		if r.Status == http.StatusCreated {
			var c ws04Cred
			r.JSON(t, &c)
			admin.Delete(t, "/service-accounts/"+c.ID)
			t.Fatalf("created a key on the backend application: %s", r.Body)
		}
		ws04Expect(t, r, 409, "CONFLICT", "binding")
	})
}

// TestWS04_ExpiryBoundaries: IAMKit accepts 1h..8760h or "never".
func TestWS04_ExpiryBoundaries(t *testing.T) {
	admin := ws04Admin(t)
	ok := []struct {
		in   string
		want time.Duration
	}{
		{"1h", time.Hour},
		{"60m", time.Hour},
		{"720h", 720 * time.Hour},
		{"8760h", 8760 * time.Hour},
	}
	for _, tc := range ok {
		t.Run("valid "+tc.in, func(t *testing.T) {
			before := time.Now()
			c := ws04Create(t, map[string]any{"name": Uniq("ws04-exp"), "expires_in": tc.in})
			d := c.ExpiresAt.Sub(before)
			// IAMKit truncates expires_at to the hour.
			if d < tc.want-time.Hour-time.Minute || d > tc.want+time.Hour {
				t.Fatalf("expires_in %s → expires_at %s (%s from now)", tc.in, c.ExpiresAt, d)
			}
			if r := Bearer(FX(t).URLs.Gateway, c.Secret).Get(t, "/models"); r.Status != 200 {
				t.Fatalf("fresh %s key on gateway: %d", tc.in, r.Status)
			}
		})
	}
	t.Run("valid never", func(t *testing.T) {
		c := ws04Create(t, map[string]any{"name": Uniq("ws04-never"), "expires_in": "never"})
		if c.ExpiresAt.Before(time.Now().Add(50 * 365 * 24 * time.Hour)) {
			t.Fatalf("never → expires_at %s; want far future", c.ExpiresAt)
		}
	})
	bad := []struct{ in, msg string }{
		{"59m", "at least 1h"},
		{"3599s", "at least 1h"},
		{"0s", "at least 1h"},
		{"-1h", "at least 1h"},
		{"-8760h", "at least 1h"},
		{"8761h", "at most 8760h"},
		{"87600h", "at most 8760h"},
		{"garbage", "Go duration"},
		{"1y", "Go duration"},
		{"NEVER", "Go duration"},
		{"2020-01-01T00:00:00Z", "Go duration"},
	}
	for _, tc := range bad {
		t.Run("invalid "+tc.in, func(t *testing.T) {
			ws04Expect(t, admin.Post(t, "/service-accounts", map[string]any{"name": "ws04-bad", "expires_in": tc.in}), 400, "VALIDATION", tc.msg)
		})
	}
}

// TestWS04_ListShowsExpiry: the list must tell the operator when a key expires.
// The console's "Expires In" column renders `expires_in || "never"`.
func TestWS04_ListShowsExpiry(t *testing.T) {
	c := ws04Create(t, map[string]any{"name": Uniq("ws04-listexp"), "expires_in": "1h"})
	a, ok := ws04Find(ws04List(t), c.ID)
	if !ok {
		t.Fatal("not listed")
	}
	if (a.ExpiresIn == nil || *a.ExpiresIn == "") && (a.ExpiresAt == nil || *a.ExpiresAt == "") {
		t.Fatalf("FINDING WS04-2: listed 1h key has no expiry information (expires_in=%v, no expires_at); console shows \"never\"", a.ExpiresIn)
	}
}

// TestWS04_NamesUnicodeLongDuplicate: names are stored verbatim; duplicates get distinct ids.
func TestWS04_NamesUnicodeLongDuplicate(t *testing.T) {
	uni := Uniq("ws04-ñandú-🔑-名前")
	long := "ws04-long-" + strings.Repeat("x", 250)
	dup := Uniq("ws04-dup")
	c1 := ws04Create(t, map[string]any{"name": uni})
	c2 := ws04Create(t, map[string]any{"name": long})
	d1 := ws04Create(t, map[string]any{"name": dup})
	d2 := ws04Create(t, map[string]any{"name": dup})
	if d1.ID == d2.ID || d1.Secret == d2.Secret {
		t.Fatal("duplicate names produced identical credentials")
	}
	list := ws04List(t)
	for id, want := range map[string]string{c1.ID: uni, c2.ID: long, d1.ID: dup, d2.ID: dup} {
		a, ok := ws04Find(list, id)
		if !ok || a.Name != want {
			t.Errorf("account %s: listed=%v name=%q want %q", id, ok, a.Name, want)
		}
	}
}

// TestWS04_ConcurrentCreate: parallel creates all succeed with unique ids/secrets.
func TestWS04_ConcurrentCreate(t *testing.T) {
	admin := ws04Admin(t)
	const n = 8
	var mu sync.Mutex
	ids, secrets := map[string]bool{}, map[string]bool{}
	var wg sync.WaitGroup
	errs := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var r Resp
			var err error
			for try := 0; try < 30; try++ {
				r, err = admin.c.DoCtx(context.Background(), "POST", "/service-accounts", map[string]any{"name": fmt.Sprintf("ws04-conc-%d", i)})
				if err != nil || r.Status != http.StatusBadGateway {
					break
				}
				time.Sleep(3 * time.Second)
			}
			if err != nil || r.Status != 201 {
				errs <- fmt.Sprintf("create %d: %v %d %s", i, err, r.Status, r.Body)
				return
			}
			var c ws04Cred
			r.JSON(t, &c)
			mu.Lock()
			ids[c.ID], secrets[c.Secret] = true, true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	close(errs)
	for id := range ids {
		id := id
		t.Cleanup(func() { admin.Delete(t, "/service-accounts/"+id) })
	}
	for e := range errs {
		t.Error(e)
	}
	if len(ids) != n || len(secrets) != n {
		t.Fatalf("got %d ids / %d secrets; want %d unique", len(ids), len(secrets), n)
	}
}

// TestWS04_RevokeImmediate: revocation applies on the very next request even though
// the server cached the exchanged machine token.
func TestWS04_RevokeImmediate(t *testing.T) {
	f := FX(t)
	admin := ws04Admin(t)
	c := ws04Create(t, map[string]any{"name": Uniq("ws04-revoke"), "permissions": []string{"freerouter:gateway:invoke", "freerouter:providers:read"}})
	gw := Bearer(f.URLs.Gateway, c.Secret)
	api := rl(Bearer(f.URLs.API, c.Secret))
	for i := 0; i < 3; i++ { // warm the machine-token cache
		if r := gw.Get(t, "/models"); r.Status != 200 {
			t.Fatalf("pre-revoke /v1/models: %d %s", r.Status, r.Body)
		}
	}
	if r := api.Get(t, "/providers"); r.Status != 200 {
		t.Fatalf("pre-revoke /api/v1/providers: %d", r.Status)
	}
	if r := admin.Delete(t, "/service-accounts/"+c.ID); r.Status != http.StatusNoContent {
		t.Fatalf("revoke: want 204, got %d %s", r.Status, r.Body)
	}
	ws04Expect(t, gw.Get(t, "/models"), 401, "UNAUTHORIZED", "")
	ws04Expect(t, api.Get(t, "/providers"), 401, "UNAUTHORIZED", "")
	ws04Expect(t, gw.Post(t, "/chat/completions", Chat("e2e-ok", "x", false)), 401, "UNAUTHORIZED", "")
	if _, ok := ws04Find(ws04List(t), c.ID); ok {
		t.Fatal("revoked account still listed")
	}
	// IAMKit refuses to mint new tokens for it as well (fresh secret-exchange path).
	x := &Client{Base: f.URLs.Gateway, Headers: map[string]string{"X-Api-Key": c.Secret}, HTTP: httpClient}
	ws04Expect(t, x.Get(t, "/models"), 401, "UNAUTHORIZED", "")
}

// TestWS04_RevokeEdgeCases: unknown / malformed / repeated revokes.
func TestWS04_RevokeEdgeCases(t *testing.T) {
	admin := ws04Admin(t)
	ws04Expect(t, admin.Delete(t, "/service-accounts/00000000-0000-0000-0000-000000000000"), 404, "NOT_FOUND", "")
	ws04Expect(t, admin.Delete(t, "/service-accounts/not-a-uuid"), 404, "NOT_FOUND", "")
	if r := admin.Delete(t, "/service-accounts/"); r.Status != http.StatusMethodNotAllowed && r.Status != http.StatusNotFound {
		t.Fatalf("DELETE without id: want 404/405, got %d %s", r.Status, r.Body)
	}
	ws04Expect(t, rl(Anon(FX(t).URLs.API)).Delete(t, "/service-accounts/"+FX(t).Personas["gw_key"].ID), 401, "UNAUTHORIZED", "")

	c := ws04Create(t, map[string]any{"name": Uniq("ws04-rerevoke")})
	if r := admin.Delete(t, "/service-accounts/"+c.ID); r.Status != 204 {
		t.Fatalf("first revoke: %d %s", r.Status, r.Body)
	}
	r := admin.Delete(t, "/service-accounts/"+c.ID)
	if r.Status != 204 && r.Status != 404 {
		t.Fatalf("second revoke: want 204 (idempotent) or 404, got %d %s", r.Status, r.Body)
	}
}

// TestWS04_ExpiredKey: an expired key is rejected everywhere.
func TestWS04_ExpiredKey(t *testing.T) {
	f := FX(t)
	s := f.Personas["expired_key"].Secret
	ws04Expect(t, Bearer(f.URLs.Gateway, s).Get(t, "/models"), 401, "UNAUTHORIZED", "")
	ws04Expect(t, Bearer(f.URLs.Gateway, s).Post(t, "/chat/completions", Chat("e2e-ok", "x", false)), 401, "UNAUTHORIZED", "")
	ws04Expect(t, rl(Bearer(f.URLs.API, s)).Get(t, "/service-accounts"), 401, "UNAUTHORIZED", "")
	ws04Expect(t, Bearer(f.URLs.Gateway, "ik_svc_doesnotexist").Get(t, "/models"), 401, "UNAUTHORIZED", "")
}

// ws04MgmtAccounts lists every service account of the environment via the management API.
func ws04MgmtAccounts(t *testing.T) []ws04Account {
	t.Helper()
	f := FX(t)
	var all []ws04Account
	for off := 0; ; off += 100 {
		r := rl(Management(t)).Get(t, fmt.Sprintf("/environments/%s/service-accounts?limit=100&offset=%d", f.IAMKit.EnvironmentID, off))
		if r.Status != 200 {
			t.Fatalf("management list service accounts: %d %s", r.Status, r.Body)
		}
		var p struct {
			Items []ws04Account
			Page  struct{ Total int }
		}
		r.JSON(t, &p)
		all = append(all, p.Items...)
		if len(p.Items) == 0 || len(all) >= p.Page.Total {
			return all
		}
	}
}

// ws04BackendAppID resolves the application of FreeRouter's backend account via management.
func ws04BackendAppID(t *testing.T) string {
	t.Helper()
	for _, a := range ws04MgmtAccounts(t) {
		if a.ID == FX(t).Boundary["backend_service_account_id"] {
			return a.ApplicationID
		}
	}
	t.Fatal("backend service account not found via management API")
	return ""
}

// TestWS04_Boundary: accounts outside FreeRouter's resource are invisible and unrevocable.
func TestWS04_Boundary(t *testing.T) {
	f := FX(t)
	admin := ws04Admin(t)
	backendID := f.Boundary["backend_service_account_id"]
	if backendID == "" {
		t.Fatal("fixtures.boundary.backend_service_account_id missing")
	}

	// A foreign account on the IAM resource, created out-of-band.
	envp := "/environments/" + f.IAMKit.EnvironmentID
	r := rl(Management(t)).Post(t, envp+"/service-accounts", map[string]any{
		"name": Uniq("ws04-foreign-iam"), "application_id": ws04BackendAppID(t),
		"resource_id": f.Boundary["iam_resource_id"], "permissions": []string{"iam:users:read"},
	})
	if r.Status != 201 && r.Status != 200 {
		t.Fatalf("management create foreign account: %d %s", r.Status, r.Body)
	}
	var foreign ws04Cred
	r.JSON(t, &foreign)
	t.Cleanup(func() { rl(Management(t)).Delete(t, envp+"/service-accounts/"+foreign.ID) })

	for _, a := range ws04List(t) {
		if a.ID == backendID || a.ID == foreign.ID {
			t.Errorf("out-of-boundary account listed: %+v", a)
		}
		if a.ResourceID != f.IAMKit.ResourceID {
			t.Errorf("listed account on foreign resource: %+v", a)
		}
	}
	for _, id := range []string{backendID, foreign.ID} {
		ws04Expect(t, admin.Delete(t, "/service-accounts/"+id), 404, "NOT_FOUND", "")
	}
	// Both must still be alive (not revoked by the rejected call).
	mg := ws04MgmtAccounts(t)
	for _, id := range []string{backendID, foreign.ID} {
		a, ok := ws04Find(mg, id)
		if !ok {
			t.Errorf("account %s missing after rejected revoke", id)
		} else if a.RevokedAt != nil {
			t.Errorf("account %s was revoked through FreeRouter", id)
		}
	}
	// FreeRouter's backend stays functional (it serves this very list).
	if r := admin.Get(t, "/service-accounts"); r.Status != 200 {
		t.Fatalf("backend broke after boundary probes: %d %s", r.Status, r.Body)
	}
}

// TestWS04_Applications: only applications usable for FreeRouter keys are offered.
func TestWS04_Applications(t *testing.T) {
	f := FX(t)
	r := ws04Admin(t).Get(t, "/service-accounts/applications")
	if r.Status != 200 {
		t.Fatalf("applications: %d %s", r.Status, r.Body)
	}
	var apps []struct {
		ID, Name string
		Active   bool
	}
	r.JSON(t, &apps)
	var haveGateway bool
	for _, a := range apps {
		if a.ID == f.IAMKit.ApplicationID {
			haveGateway = a.Name == "FreeRouter Gateway" && a.Active
		}
	}
	if !haveGateway {
		t.Fatalf("FreeRouter Gateway application missing or inactive: %s", r.Body)
	}
	// RBAC
	if r := rl(Bearer(f.URLs.API, f.Personas["gw_key"].Secret)).Get(t, "/service-accounts/applications"); r.Status != 403 {
		t.Fatalf("gw_key applications: want 403, got %d", r.Status)
	}
	if r := rl(AsUser(t, "viewer")).Get(t, "/service-accounts/applications"); r.Status != 200 {
		t.Fatalf("viewer applications: want 200, got %d", r.Status)
	}
	backendApp := ws04BackendAppID(t)
	for _, a := range apps {
		if a.ID == backendApp {
			t.Fatalf("FINDING WS04-3: applications lists %q (%s), FreeRouter's backend app bound to the IAM resource; creating a key on it fails with 409. Full list: %s", a.Name, a.ID, r.Body)
		}
	}
}

// TestWS04_UserRBAC: user personas follow service-accounts:read/write.
func TestWS04_UserRBAC(t *testing.T) {
	viewer := rl(AsUser(t, "viewer"))
	if r := viewer.Get(t, "/service-accounts"); r.Status != 200 {
		t.Fatalf("viewer list: %d %s", r.Status, r.Body)
	}
	ws04Expect(t, viewer.Post(t, "/service-accounts", map[string]any{"name": "ws04-viewer"}), 403, "FORBIDDEN", "")
	ws04Expect(t, viewer.Delete(t, "/service-accounts/"+FX(t).Personas["gw_key"].ID), 403, "FORBIDDEN", "")
	ws04Expect(t, rl(AsUser(t, "providers_only")).Get(t, "/service-accounts"), 403, "FORBIDDEN", "")
	ws04Expect(t, rl(Bearer(FX(t).URLs.API, FX(t).Personas["noperm_key"].Secret)).Get(t, "/service-accounts"), 403, "FORBIDDEN", "")
	ws04Expect(t, rl(Bearer(FX(t).URLs.API, FX(t).Personas["gw_key"].Secret)).Get(t, "/service-accounts"), 403, "FORBIDDEN", "")

	admin := rl(AsUser(t, "admin"))
	r := admin.Post(t, "/service-accounts", map[string]any{"name": Uniq("ws04-byuser")})
	if r.Status != 201 {
		t.Fatalf("admin user create: %d %s", r.Status, r.Body)
	}
	var c ws04Cred
	r.JSON(t, &c)
	if r := admin.Delete(t, "/service-accounts/"+c.ID); r.Status != 204 {
		t.Fatalf("admin user revoke: %d %s", r.Status, r.Body)
	}
}

// TestWS04_NoPrivilegeEscalation: a credential holding only service-accounts:write
// must not be able to mint a key with permissions it does not itself hold.
func TestWS04_NoPrivilegeEscalation(t *testing.T) {
	f := FX(t)
	c := ws04Create(t, map[string]any{"name": Uniq("ws04-sawrite"), "permissions": []string{"freerouter:service-accounts:write"}})
	r := rl(Bearer(f.URLs.API, c.Secret)).Post(t, "/service-accounts", map[string]any{
		"name": Uniq("ws04-escalated"), "permissions": []string{"freerouter:users:write", "freerouter:roles:write", "freerouter:providers:write"},
	})
	if r.Status == http.StatusCreated {
		var e ws04Cred
		r.JSON(t, &e)
		ws04Admin(t).Delete(t, "/service-accounts/"+e.ID)
		t.Fatalf("FINDING WS04-1: key with only service-accounts:write minted a key with users:write/roles:write/providers:write (201 %s)", e.ID)
	}
	if r.Status != http.StatusForbidden {
		t.Fatalf("want 403, got %d %s", r.Status, r.Body)
	}
}
