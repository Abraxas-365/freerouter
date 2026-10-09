//go:build e2e

package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// WS-01 — authentication & session behaviour of the FreeRouter API and of
// the IAMKit identity endpoints the console uses (/identity/v1/login,
// /refresh, /logout).

const (
	ws01IAMInvalid  = "invalid credentials or access token"
	ws01BadToken    = "invalid or expired token"
	ws01NoBearer    = "missing or invalid bearer token"
	ws01BadRefresh  = "invalid refresh token"
	ws01ReplayError = "refresh token replay revoked session"
)

// ── helpers ─────────────────────────────────────────────────────────

type ws01Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// ws01Post posts to IAMKit /identity/v1, retrying while IAMKit's per-IP
// limiter (shared by every workstream) answers 429.
func ws01Post(t *testing.T, path string, body any, bearer string) Resp {
	t.Helper()
	c := Anon(FX(t).URLs.IAMKit + "/identity/v1")
	if bearer != "" {
		c = Bearer(FX(t).URLs.IAMKit+"/identity/v1", bearer)
	}
	var r Resp
	for i := 0; i < 15; i++ {
		r = c.Post(t, path, body)
		if r.Status != http.StatusTooManyRequests {
			return r
		}
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("%s still rate limited: %d %s", path, r.Status, r.Body)
	return r
}

func ws01LoginBody(t *testing.T, email, password string) map[string]string {
	f := FX(t)
	return map[string]string{
		"environment_id": f.IAMKit.EnvironmentID, "organization_id": f.IAMKit.OrganizationID,
		"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
		"email": email, "password": password,
	}
}

func ws01Login(t *testing.T, email, password string) Resp {
	t.Helper()
	return ws01Post(t, "/login", ws01LoginBody(t, email, password), "")
}

func ws01MustLogin(t *testing.T, email, password string) ws01Tokens {
	t.Helper()
	r := ws01Login(t, email, password)
	if r.Status != 200 {
		t.Fatalf("login %q: %d %s", email, r.Status, r.Body)
	}
	var tok ws01Tokens
	r.JSON(t, &tok)
	return tok
}

func ws01Refresh(t *testing.T, refresh string) Resp {
	t.Helper()
	f := FX(t)
	return ws01Post(t, "/refresh", map[string]string{
		"environment_id": f.IAMKit.EnvironmentID, "organization_id": f.IAMKit.OrganizationID,
		"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
		"refresh_token": refresh,
	}, "")
}

func ws01Logout(t *testing.T, access, audience string) Resp {
	t.Helper()
	f := FX(t)
	return ws01Post(t, "/logout", map[string]string{"environment_id": f.IAMKit.EnvironmentID, "audience": audience}, access)
}

// ws01ErrMsg extracts error.message from either the fiberauth/IAMKit
// envelope ({"error":{"message"}}) or FreeRouter's errx shape ({"message"}).
func ws01ErrMsg(t *testing.T, r Resp) string {
	t.Helper()
	var out struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.Body, &out)
	if out.Error.Message != "" {
		return out.Error.Message
	}
	return out.Message
}

func ws01Expect(t *testing.T, what string, r Resp, status int, msg string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("%s: want %d, got %d %s", what, status, r.Status, r.Body)
	}
	if msg != "" {
		if got := ws01ErrMsg(t, r); got != msg {
			t.Fatalf("%s: want message %q, got %q (%s)", what, msg, got, r.Body)
		}
	}
}

func ws01Claims(t *testing.T, jwt string) map[string]any {
	t.Helper()
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", jwt)
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse JWT payload: %v", err)
	}
	return m
}

// ws01User creates a FreeRouter user (via the admin API) holding the viewer
// role — IAMKit refuses logins of users with no grant on the resource — and
// permanently deletes it at IAMKit on cleanup.
func ws01User(t *testing.T, prefix, password string) (id, email string) {
	t.Helper()
	f := FX(t)
	email = strings.ToLower(Uniq("ws01-"+prefix)) + "@e2e.test"
	r := AdminAPI(t).Post(t, "/access/users", map[string]string{"email": email, "name": "WS01 " + prefix, "password": password})
	if r.Status != http.StatusCreated && r.Status != http.StatusOK {
		t.Fatalf("create user: %d %s", r.Status, r.Body)
	}
	id, _ = r.Map(t)["id"].(string)
	t.Cleanup(func() {
		Management(t).Delete(t, "/environments/"+f.IAMKit.EnvironmentID+"/users/"+id+"/permanent")
	})
	a := AdminAPI(t).Post(t, "/access/role-assignments", map[string]string{"user_id": id, "role_id": f.Roles["viewer"]})
	if a.Status != http.StatusNoContent && a.Status != http.StatusOK && a.Status != http.StatusCreated {
		t.Fatalf("assign viewer role: %d %s", a.Status, a.Body)
	}
	return id, email
}

// ── login ───────────────────────────────────────────────────────────

func TestWS01_Login(t *testing.T) {
	f := FX(t)
	viewer := f.Personas["viewer"]

	t.Run("valid credentials issue a bearer token pair scoped to FreeRouter", func(t *testing.T) {
		tok := ws01MustLogin(t, viewer.Email, viewer.Password)
		if tok.AccessToken == "" || tok.RefreshToken == "" {
			t.Fatalf("missing tokens: %+v", tok)
		}
		if tok.TokenType != "Bearer" || tok.ExpiresIn != 900 {
			t.Fatalf("want Bearer/900s, got %q/%d", tok.TokenType, tok.ExpiresIn)
		}
		c := ws01Claims(t, tok.AccessToken)
		if c["sub"] != viewer.UserID || c["application_id"] != f.IAMKit.ApplicationID || c["resource_id"] != f.IAMKit.ResourceID {
			t.Fatalf("unexpected claims: %v", c)
		}
		if aud, _ := c["aud"].([]any); len(aud) != 1 || aud[0] != f.IAMKit.Audience {
			t.Fatalf("aud = %v, want [%s]", c["aud"], f.IAMKit.Audience)
		}
		r := Bearer(f.URLs.API, tok.AccessToken).Get(t, "/providers")
		ws01Expect(t, "viewer lists providers", r, 200, "")
	})

	// Every failure must look identical: no account enumeration.
	for _, tc := range []struct{ name, email, password string }{
		{"wrong password", viewer.Email, "definitely-wrong-password"},
		{"unknown email", "nobody-ws01@e2e.test", "e2e-password-12345"},
		{"empty email and password", "", ""},
		{"empty password", viewer.Email, ""},
		{"password with trailing space", viewer.Email, viewer.Password + " "},
		{"password case-changed", viewer.Email, strings.ToUpper(viewer.Password)},
		{"suspended user", f.Personas["suspended"].Email, f.Personas["suspended"].Password},
		{"user with no grant on the resource", f.Personas["noperm"].Email, f.Personas["noperm"].Password},
	} {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			ws01Expect(t, tc.name, ws01Login(t, tc.email, tc.password), 401, ws01IAMInvalid)
		})
	}

	t.Run("email is case-insensitive", func(t *testing.T) {
		tok := ws01MustLogin(t, strings.ToUpper(viewer.Email), viewer.Password)
		if sub := ws01Claims(t, tok.AccessToken)["sub"]; sub != viewer.UserID {
			t.Fatalf("sub = %v, want %s", sub, viewer.UserID)
		}
	})

	t.Run("email surrounding whitespace is trimmed", func(t *testing.T) {
		tok := ws01MustLogin(t, "  "+viewer.Email+"\t", viewer.Password)
		if sub := ws01Claims(t, tok.AccessToken)["sub"]; sub != viewer.UserID {
			t.Fatalf("sub = %v, want %s", sub, viewer.UserID)
		}
	})
}

// bcrypt only hashes the first 72 bytes; IAMKit must reject longer
// passwords at creation instead of silently truncating them.
func TestWS01_PasswordByteLimit(t *testing.T) {
	ascii72 := strings.Repeat("p", 72)

	t.Run("73-byte password is rejected at creation", func(t *testing.T) {
		r := AdminAPI(t).Post(t, "/access/users", map[string]string{
			"email": strings.ToLower(Uniq("ws01-pw73")) + "@e2e.test", "name": "WS01 pw73", "password": ascii72 + "x",
		})
		ws01Expect(t, "create with 73-byte password", r, 400, "password must be at most 72 characters long")
	})

	t.Run("multibyte password over 72 bytes but under 72 chars is rejected", func(t *testing.T) {
		pw := strings.Repeat("€", 25) // 25 chars, 75 bytes
		r := AdminAPI(t).Post(t, "/access/users", map[string]string{
			"email": strings.ToLower(Uniq("ws01-pwmb")) + "@e2e.test", "name": "WS01 pwmb", "password": pw,
		})
		if r.Status != 400 {
			t.Fatalf("want 400 for 75-byte password, got %d %s", r.Status, r.Body)
		}
	})

	t.Run("exactly 72 bytes works and no prefix/suffix variant does", func(t *testing.T) {
		_, email := ws01User(t, "pw72", ascii72)
		ws01MustLogin(t, email, ascii72)
		ws01Expect(t, "72+1 bytes at login", ws01Login(t, email, ascii72+"x"), 401, ws01IAMInvalid)
		ws01Expect(t, "71-byte prefix", ws01Login(t, email, ascii72[:71]), 401, ws01IAMInvalid)
	})

	t.Run("exactly 72 bytes of multibyte runes works", func(t *testing.T) {
		pw := strings.Repeat("€", 24) // 72 bytes
		_, email := ws01User(t, "pw72mb", pw)
		ws01MustLogin(t, email, pw)
		ws01Expect(t, "last rune changed", ws01Login(t, email, strings.Repeat("€", 23)+"£"), 401, ws01IAMInvalid)
	})
}

// ── session lifecycle ───────────────────────────────────────────────

func TestWS01_SuspendedMidSession(t *testing.T) {
	f := FX(t)
	id, email := ws01User(t, "susp", "e2e-password-12345")
	tok := ws01MustLogin(t, email, "e2e-password-12345")
	api := Bearer(f.URLs.API, tok.AccessToken)
	ws01Expect(t, "before suspension", api.Get(t, "/providers"), 200, "")

	if r := AdminAPI(t).Delete(t, "/access/users/"+id); r.Status != http.StatusNoContent {
		t.Fatalf("suspend: %d %s", r.Status, r.Body)
	}
	// Introspection is online, so the very next call must fail.
	ws01Expect(t, "access token after suspension", api.Get(t, "/providers"), 401, ws01BadToken)
	ws01Expect(t, "gateway after suspension", Bearer(f.URLs.Gateway, tok.AccessToken).Get(t, "/models"), 401, ws01BadToken)
	ws01Expect(t, "refresh after suspension", ws01Refresh(t, tok.RefreshToken), 401, ws01BadRefresh)
	ws01Expect(t, "login after suspension", ws01Login(t, email, "e2e-password-12345"), 401, ws01IAMInvalid)
}

func TestWS01_RefreshRotation(t *testing.T) {
	f := FX(t)
	v := f.Personas["viewer"]

	t.Run("refresh rotates the refresh token and both access tokens stay valid", func(t *testing.T) {
		first := ws01MustLogin(t, v.Email, v.Password)
		r := ws01Refresh(t, first.RefreshToken)
		ws01Expect(t, "refresh", r, 200, "")
		var second ws01Tokens
		r.JSON(t, &second)
		if second.AccessToken == "" || second.RefreshToken == "" || second.RefreshToken == first.RefreshToken {
			t.Fatalf("refresh did not rotate: %+v", second)
		}
		if sub := ws01Claims(t, second.AccessToken)["sub"]; sub != v.UserID {
			t.Fatalf("refreshed token sub = %v", sub)
		}
		ws01Expect(t, "new access token", Bearer(f.URLs.API, second.AccessToken).Get(t, "/providers"), 200, "")
		ws01Expect(t, "previous access token (same session)", Bearer(f.URLs.API, first.AccessToken).Get(t, "/providers"), 200, "")
		ws01Logout(t, second.AccessToken, f.IAMKit.Audience)
	})

	t.Run("replaying a rotated refresh token revokes the whole session", func(t *testing.T) {
		first := ws01MustLogin(t, v.Email, v.Password)
		r := ws01Refresh(t, first.RefreshToken)
		ws01Expect(t, "refresh", r, 200, "")
		var second ws01Tokens
		r.JSON(t, &second)

		ws01Expect(t, "replay", ws01Refresh(t, first.RefreshToken), 401, ws01ReplayError)
		ws01Expect(t, "access token after replay", Bearer(f.URLs.API, second.AccessToken).Get(t, "/providers"), 401, ws01BadToken)
		ws01Expect(t, "rotated refresh token after replay", ws01Refresh(t, second.RefreshToken), 401, ws01BadRefresh)
	})

	for _, tc := range []struct{ name, token string }{
		{"garbage", "garbage"},
		{"empty", ""},
		{"access token used as refresh token", "eyJhbGciOiJSUzI1NiJ9.e30.sig"},
	} {
		t.Run("rejects "+tc.name+" refresh token", func(t *testing.T) {
			r := ws01Refresh(t, tc.token)
			if r.Status != 401 && r.Status != 400 {
				t.Fatalf("want 401/400, got %d %s", r.Status, r.Body)
			}
		})
	}
}

func TestWS01_LogoutRevokes(t *testing.T) {
	f := FX(t)
	v := f.Personas["viewer"]
	tok := ws01MustLogin(t, v.Email, v.Password)
	ws01Expect(t, "before logout", Bearer(f.URLs.API, tok.AccessToken).Get(t, "/providers"), 200, "")

	r := ws01Logout(t, tok.AccessToken, f.IAMKit.Audience)
	if r.Status != http.StatusNoContent {
		t.Fatalf("logout: want 204, got %d %s", r.Status, r.Body)
	}
	ws01Expect(t, "access token after logout", Bearer(f.URLs.API, tok.AccessToken).Get(t, "/providers"), 401, ws01BadToken)
	ws01Expect(t, "refresh token after logout", ws01Refresh(t, tok.RefreshToken), 401, ws01BadRefresh)
	ws01Expect(t, "second logout with revoked token", ws01Logout(t, tok.AccessToken, f.IAMKit.Audience), 401, ws01IAMInvalid)
}

// ── API credential handling ─────────────────────────────────────────

func TestWS01_APICredentials(t *testing.T) {
	f := FX(t)
	gw := f.Personas["gw_key"].Secret
	userTok := ws01MustLogin(t, f.Personas["viewer"].Email, f.Personas["viewer"].Password)
	t.Cleanup(func() { ws01Logout(t, userTok.AccessToken, f.IAMKit.Audience) })

	parts := strings.Split(userTok.AccessToken, ".")
	// Same header+payload, signature of a different (unsigned) shape.
	badSig := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString([]byte("not-the-signature"))
	// Payload swapped to claim admin while keeping the original signature.
	claims := ws01Claims(t, userTok.AccessToken)
	claims["sub"] = f.Personas["admin"].UserID
	claims["permissions"] = f.Permissions
	pb, _ := json.Marshal(claims)
	forgedPayload := parts[0] + "." + base64.RawURLEncoding.EncodeToString(pb) + "." + parts[2]
	algNone := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(pb) + "."
	backendSecret := ws01EnvValue(t, "IAMKIT_SERVICE_SECRET")

	type tc struct {
		name    string
		headers map[string]string
		status  int
		msg     string
	}
	cases := []tc{
		{"no credentials", nil, 401, ws01NoBearer},
		{"empty Authorization", map[string]string{"Authorization": ""}, 401, ws01NoBearer},
		{"Bearer without token", map[string]string{"Authorization": "Bearer"}, 401, ws01NoBearer},
		{"Bearer with two tokens", map[string]string{"Authorization": "Bearer a b"}, 401, ws01NoBearer},
		{"Basic auth", map[string]string{"Authorization": "Basic eDp5"}, 401, ws01NoBearer},
		{"raw key without scheme", map[string]string{"Authorization": gw}, 401, ws01NoBearer},
		{"Bearer garbage", map[string]string{"Authorization": "Bearer garbage"}, 401, ws01BadToken},
		{"Bearer ik_svc_ unknown secret", map[string]string{"Authorization": "Bearer ik_svc_ws01-not-a-real-secret-0000000000"}, 401, ws01BadToken},
		{"Bearer ik_svc_ empty secret", map[string]string{"Authorization": "Bearer ik_svc_"}, 401, ws01BadToken},
		{"expired service account", map[string]string{"Authorization": "Bearer " + f.Personas["expired_key"].Secret}, 401, ws01BadToken},
		{"JWT with tampered signature", map[string]string{"Authorization": "Bearer " + badSig}, 401, ws01BadToken},
		{"JWT with forged payload", map[string]string{"Authorization": "Bearer " + forgedPayload}, 401, ws01BadToken},
		{"alg=none JWT", map[string]string{"Authorization": "Bearer " + algNone}, 401, ws01BadToken},
		{"backend service account (IAM resource, other audience)", map[string]string{"Authorization": "Bearer " + backendSecret}, 401, ws01BadToken},
		{"management key as bearer", map[string]string{"Authorization": "Bearer " + f.IAMKit.ManagementKey}, 401, ws01BadToken},
		{"refresh token as bearer", map[string]string{"Authorization": "Bearer " + userTok.RefreshToken}, 401, ws01BadToken},
		{"valid key, lowercase scheme", map[string]string{"Authorization": "bearer " + gw}, 200, ""},
		{"valid key via X-Api-Key", map[string]string{"X-Api-Key": gw}, 200, ""},
		{"valid key via X-Api-Key with whitespace", map[string]string{"X-Api-Key": "  " + gw + "  "}, 200, ""},
		// viewer has no gateway:invoke: authenticated (not 401) but forbidden.
		{"user JWT via X-Api-Key", map[string]string{"X-Api-Key": userTok.AccessToken}, 403, "insufficient permissions"},
		{"garbage X-Api-Key", map[string]string{"X-Api-Key": "garbage"}, 401, ws01BadToken},
		// Authorization wins when both are present.
		{"both: Authorization garbage, X-Api-Key valid", map[string]string{"Authorization": "Bearer garbage", "X-Api-Key": gw}, 401, ws01BadToken},
		{"both: Authorization valid, X-Api-Key garbage", map[string]string{"Authorization": "Bearer " + gw, "X-Api-Key": "garbage"}, 200, ""},
		{"blank Authorization falls back to X-Api-Key", map[string]string{"Authorization": "   ", "X-Api-Key": gw}, 200, ""},
	}
	for _, c := range cases {
		t.Run("gateway /v1/models: "+c.name, func(t *testing.T) {
			cl := &Client{Base: f.URLs.Gateway, Headers: c.headers, HTTP: httpClient}
			r := cl.Get(t, "/models")
			ws01Expect(t, c.name, r, c.status, c.msg)
			if r.Status == 401 {
				if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
					t.Fatalf("401 Cache-Control = %q, want no-store", cc)
				}
				if r.Elapsed > 5*time.Second {
					t.Fatalf("rejection took %s (iamCallTimeout is 5s)", r.Elapsed)
				}
			}
		})
	}

	// Same credential matrix on the management API (gw key → 403 there).
	for _, c := range []tc{
		{"no credentials", nil, 401, ws01NoBearer},
		{"Bearer garbage", map[string]string{"Authorization": "Bearer garbage"}, 401, ws01BadToken},
		{"Bearer ik_svc_ unknown secret", map[string]string{"Authorization": "Bearer ik_svc_ws01-not-a-real-secret-0000000000"}, 401, ws01BadToken},
		{"JWT with forged payload", map[string]string{"Authorization": "Bearer " + forgedPayload}, 401, ws01BadToken},
		{"backend service account", map[string]string{"Authorization": "Bearer " + backendSecret}, 401, ws01BadToken},
		{"user JWT via X-Api-Key", map[string]string{"X-Api-Key": userTok.AccessToken}, 200, ""},
		{"admin key via X-Api-Key", map[string]string{"X-Api-Key": f.Personas["admin_key"].Secret}, 200, ""},
		{"gw key is authenticated but forbidden", map[string]string{"X-Api-Key": gw}, 403, "insufficient permissions"},
	} {
		t.Run("api /providers: "+c.name, func(t *testing.T) {
			cl := &Client{Base: f.URLs.API, Headers: c.headers, HTTP: httpClient}
			ws01Expect(t, c.name, cl.Get(t, "/providers"), c.status, c.msg)
		})
	}

	t.Run("oversized Authorization header is refused, not processed", func(t *testing.T) {
		cl := Bearer(f.URLs.API, strings.Repeat("a", 20_000))
		if r := cl.Get(t, "/providers"); r.Status != http.StatusRequestHeaderFieldsTooLarge {
			t.Fatalf("want 431, got %d", r.Status)
		}
	})

	t.Run("public routes need no credentials", func(t *testing.T) {
		ws01Expect(t, "health", Anon(f.URLs.Server).Get(t, "/health"), 200, "")
	})
}

// ws01EnvValue reads a key from e2e/.run/.env (needed for the backend
// service-account secret, which is not in fixtures.json).
func ws01EnvValue(t *testing.T, key string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", ".run", ".env"))
	if err != nil {
		t.Skipf("e2e/.run/.env not readable: %v", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Skipf("%s not in e2e/.run/.env", key)
	return ""
}

// ── foreign application / audience tokens ───────────────────────────

// ws01App creates an IAMKit application in FreeRouter's environment, links
// it to resource, and on cleanup unlinks (best effort) and deactivates it —
// IAMKit has no application delete.
func ws01App(t *testing.T, resource string) string {
	t.Helper()
	f := FX(t)
	env := "/environments/" + f.IAMKit.EnvironmentID
	m := Management(t)
	r := m.Post(t, env+"/applications", map[string]any{"name": Uniq("ws01-foreign-app"), "redirect_uris": []string{"http://localhost/ws01/cb"}})
	if r.Status != http.StatusCreated {
		t.Fatalf("create application: %d %s", r.Status, r.Body)
	}
	app, _ := r.Map(t)["id"].(string)
	t.Cleanup(func() {
		m.Delete(t, env+"/application-resources/"+app+"/"+resource)
		if r := m.Patch(t, env+"/applications/"+app, map[string]any{"active": false}); r.Status != http.StatusNoContent {
			t.Errorf("deactivate application %s: %d %s", app, r.Status, r.Body)
		}
	})
	if r := m.Post(t, env+"/application-resources", map[string]string{"application_id": app, "resource_id": resource}); r.Status != http.StatusCreated {
		t.Fatalf("link application: %d %s", r.Status, r.Body)
	}
	return app
}

func TestWS01_ForeignApplicationJWT(t *testing.T) {
	f := FX(t)
	v := f.Personas["viewer"]

	// Same environment, same resource/audience, same user and permissions —
	// only application_id differs. FreeRouter pins IAMKIT_APPLICATION_ID.
	app := ws01App(t, f.IAMKit.ResourceID)
	body := ws01LoginBody(t, v.Email, v.Password)
	body["application_id"] = app
	r := ws01Post(t, "/login", body, "")
	ws01Expect(t, "login to foreign app", r, 200, "")
	var tok ws01Tokens
	r.JSON(t, &tok)
	t.Cleanup(func() { ws01Logout(t, tok.AccessToken, f.IAMKit.Audience) })

	c := ws01Claims(t, tok.AccessToken)
	if c["application_id"] != app || c["resource_id"] != f.IAMKit.ResourceID {
		t.Fatalf("unexpected claims: %v", c)
	}
	if perms, _ := c["permissions"].([]any); len(perms) == 0 {
		t.Fatalf("foreign-app token carries no permissions, test would be vacuous: %v", c)
	}
	ws01Expect(t, "foreign-app JWT on management API", Bearer(f.URLs.API, tok.AccessToken).Get(t, "/providers"), 401, ws01BadToken)
	ws01Expect(t, "foreign-app JWT on gateway", Bearer(f.URLs.Gateway, tok.AccessToken).Get(t, "/models"), 401, ws01BadToken)
	ws01Expect(t, "foreign-app JWT via X-Api-Key", (&Client{Base: f.URLs.API, Headers: map[string]string{"X-Api-Key": tok.AccessToken}, HTTP: httpClient}).Get(t, "/providers"), 401, ws01BadToken)
}

func TestWS01_ForeignAudienceJWT(t *testing.T) {
	f := FX(t)
	v := f.Personas["viewer"]
	iamRes := f.Boundary["iam_resource_id"]
	if iamRes == "" {
		t.Skip("boundary.iam_resource_id missing from fixtures")
	}
	env := "/environments/" + f.IAMKit.EnvironmentID
	m := Management(t)

	// A token for the environment's built-in IAM resource (audience
	// urn:iamkit:environment:<env>), issued to a fresh app for the viewer.
	app := ws01App(t, iamRes)
	g := m.Put(t, env+"/grants", map[string]any{
		"organization_id": f.IAMKit.OrganizationID, "user_id": v.UserID, "resource_id": iamRes,
		"permissions": []string{"iam:users:read"},
	})
	if g.Status != 200 && g.Status != 201 {
		t.Fatalf("grant: %d %s", g.Status, g.Body)
	}
	grant, _ := g.Map(t)["id"].(string)
	t.Cleanup(func() { m.Delete(t, env+"/grants/"+grant) })

	body := ws01LoginBody(t, v.Email, v.Password)
	body["application_id"], body["resource_id"] = app, iamRes
	r := ws01Post(t, "/login", body, "")
	ws01Expect(t, "login to IAM resource", r, 200, "")
	var tok ws01Tokens
	r.JSON(t, &tok)
	c := ws01Claims(t, tok.AccessToken)
	t.Cleanup(func() {
		aud, _ := c["aud"].([]any)
		if len(aud) > 0 {
			ws01Logout(t, tok.AccessToken, aud[0].(string))
		}
	})
	if aud, _ := c["aud"].([]any); len(aud) == 0 || aud[0] == f.IAMKit.Audience {
		t.Fatalf("expected a foreign audience, got %v", c["aud"])
	}
	ws01Expect(t, "other-audience JWT on management API", Bearer(f.URLs.API, tok.AccessToken).Get(t, "/providers"), 401, ws01BadToken)
	ws01Expect(t, "other-audience JWT on gateway", Bearer(f.URLs.Gateway, tok.AccessToken).Get(t, "/models"), 401, ws01BadToken)
}
