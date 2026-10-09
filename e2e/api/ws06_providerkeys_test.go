//go:build e2e

package api

// WS-06: provider keys — CRUD, masking, encryption at rest, OAuth keys and the
// gateway's use of keys (disabled keys skipped, deleted keys unroutable,
// rotation, base_url override, sort_order). Everything is prefixed ws06-, uses
// its own provider pointed at fakellm under /ws06/<seg>/ and is deleted in
// t.Cleanup. Upstream assertions filter fakellm GET /_requests by that prefix.

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// ── helpers ─────────────────────────────────────────────────────────

const ws06Missing = "00000000-0000-4000-8000-000000000006"

func ws06Expect(t *testing.T, r Resp, status int, code, msg string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("want %d, got %d %s", status, r.Status, r.Body)
	}
	if code == "" && msg == "" {
		return
	}
	var e struct{ Code, Message string }
	r.JSON(t, &e)
	if code != "" && e.Code != code {
		t.Fatalf("want code %q, got %q (%s)", code, e.Code, r.Body)
	}
	if msg != "" && e.Message != msg {
		t.Fatalf("want message %q, got %q", msg, e.Message)
	}
}

// ws06Seg returns a unique fakellm path segment for one test.
func ws06Seg(name string) string {
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano()%1_000_000_000)
}

// ws06Provider creates an openai provider pointed at fakellm /ws06/<seg>/openai/v1.
func ws06Provider(t *testing.T, seg string) string {
	t.Helper()
	admin := AdminAPI(t)
	r := admin.Post(t, "/providers", map[string]any{
		"name": "ws06-" + seg, "protocol": "openai", "description": "ws06",
		"base_url": FX(t).URLs.FakeLLM + "/ws06/" + seg + "/openai/v1", "streaming": true,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider: %d %s", r.Status, r.Body)
	}
	id := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/providers/"+id) })
	return id
}

// ws06Key creates a provider key from an arbitrary body (provider_id required).
func ws06Key(t *testing.T, body map[string]any) string {
	t.Helper()
	admin := AdminAPI(t)
	r := admin.Post(t, "/provider-keys", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider key: %d %s", r.Status, r.Body)
	}
	id := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/provider-keys/"+id) })
	return id
}

func ws06APIKey(t *testing.T, provider, name, token string) string {
	t.Helper()
	return ws06Key(t, map[string]any{"provider_id": provider, "name": name, "key_type": "api_key", "token": token})
}

// ws06Model creates a model mapped to provider with external id fake-ok; returns the model name.
func ws06Model(t *testing.T, provider, seg string) string {
	t.Helper()
	admin := AdminAPI(t)
	name := "ws06-" + seg
	r := admin.Post(t, "/models", map[string]any{"name": name, "family": "ws06", "description": "ws06"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create model: %d %s", r.Status, r.Body)
	}
	mid := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/models/"+mid) })
	m := admin.Post(t, "/mappings", map[string]any{
		"model_id": mid, "provider_id": provider, "external_id": "fake-ok",
		"input_price": 1.0, "output_price": 2.0, "streaming": true,
	})
	if m.Status != http.StatusCreated {
		t.Fatalf("create mapping: %d %s", m.Status, m.Body)
	}
	mapID := m.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/mappings/"+mapID) })
	return name
}

type ws06KeyView map[string]any

func ws06Get(t *testing.T, id string) ws06KeyView {
	t.Helper()
	r := AdminAPI(t).Get(t, "/provider-keys/"+id)
	if r.Status != http.StatusOK {
		t.Fatalf("get key: %d %s", r.Status, r.Body)
	}
	return r.Map(t)
}

func ws06List(t *testing.T, query string) []ws06KeyView {
	t.Helper()
	r := AdminAPI(t).Get(t, "/provider-keys?"+query)
	if r.Status != http.StatusOK {
		t.Fatalf("list keys: %d %s", r.Status, r.Body)
	}
	var out struct {
		Items []ws06KeyView
		Page  struct{ Total, Limit, Offset int }
	}
	r.JSON(t, &out)
	return out.Items
}

// ws06AssertNoSecret checks a raw response body never carries any of the secrets.
func ws06AssertNoSecret(t *testing.T, body []byte, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(string(body), s) {
			t.Fatalf("response leaks secret %q: %s", s, body)
		}
	}
	for _, f := range []string{"token", "token_ciphertext", "token_hash", "oauth_data", "access_token", "refresh_token"} {
		if strings.Contains(string(body), `"`+f+`":`) {
			t.Fatalf("response exposes field %q: %s", f, body)
		}
	}
}

var ws06ChatN int64
var ws06ChatMu sync.Mutex

// ws06Chat calls the gateway with gw_key using a unique prompt (defeats the
// response cache); retries on the shared gw_key rate limit (429).
func ws06Chat(t *testing.T, model string) Resp {
	t.Helper()
	f := FX(t)
	gw := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret)
	ws06ChatMu.Lock()
	ws06ChatN++
	n := ws06ChatN
	ws06ChatMu.Unlock()
	var r Resp
	for i := 0; i < 6; i++ {
		r = gw.Post(t, "/chat/completions", Chat(model, fmt.Sprintf("ws06 %d %d", time.Now().UnixNano(), n), false))
		if r.Status != http.StatusTooManyRequests {
			return r
		}
		time.Sleep(5 * time.Second)
	}
	return r
}

type ws06Rec struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

// ws06Upstream returns fakellm requests under /ws06/<seg>/ (only this test's provider).
func ws06Upstream(t *testing.T, seg string) []ws06Rec {
	t.Helper()
	var all []ws06Rec
	Anon(FX(t).URLs.FakeLLM).Get(t, "/_requests").JSON(t, &all)
	var out []ws06Rec
	for _, r := range all {
		if strings.HasPrefix(r.Path, "/ws06/"+seg+"/") {
			out = append(out, r)
		}
	}
	return out
}

// ws06ChatAuth performs one gateway call that must succeed and returns the
// Authorization header + path the upstream saw for it.
func ws06ChatAuth(t *testing.T, model, seg string) (auth, path string) {
	t.Helper()
	before := len(ws06Upstream(t, seg))
	r := ws06Chat(t, model)
	if r.Status != http.StatusOK {
		t.Fatalf("gateway call: %d %s", r.Status, r.Body)
	}
	recs := ws06Upstream(t, seg)
	if len(recs) != before+1 {
		t.Fatalf("want exactly 1 new upstream request under /ws06/%s/, got %d", seg, len(recs)-before)
	}
	last := recs[len(recs)-1]
	return last.Headers["Authorization"], last.Path
}

var (
	ws06DBOnce sync.Once
	ws06DB     *sql.DB
	ws06DBErr  error
)

func ws06Postgres(t *testing.T) *sql.DB {
	t.Helper()
	ws06DBOnce.Do(func() {
		ws06DB, ws06DBErr = sql.Open("postgres", FX(t).URLs.DB)
		if ws06DBErr == nil {
			ws06DBErr = ws06DB.Ping()
		}
	})
	if ws06DBErr != nil {
		t.Fatalf("postgres: %v", ws06DBErr)
	}
	return ws06DB
}

type ws06Row struct{ Cipher, Masked, Hash, KeyType string }

func ws06Stored(t *testing.T, id string) ws06Row {
	t.Helper()
	var r ws06Row
	err := ws06Postgres(t).QueryRow(
		`SELECT token_ciphertext, token_masked, token_hash, key_type FROM provider_keys WHERE id = $1`, id,
	).Scan(&r.Cipher, &r.Masked, &r.Hash, &r.KeyType)
	if err != nil {
		t.Fatalf("read provider_keys row %s: %v", id, err)
	}
	return r
}

// ws06AssertOpaque checks the stored ciphertext reveals none of the secrets in
// plain, hex or base64 form.
func ws06AssertOpaque(t *testing.T, stored string, secrets ...string) {
	t.Helper()
	low := strings.ToLower(stored)
	for _, s := range secrets {
		for _, form := range []string{s, hex.EncodeToString([]byte(s)), base64.StdEncoding.EncodeToString([]byte(s)), base64.RawURLEncoding.EncodeToString([]byte(s))} {
			if strings.Contains(stored, form) || strings.Contains(low, strings.ToLower(form)) {
				t.Fatalf("stored ciphertext contains secret %q (as %q): %s", s, form, stored)
			}
		}
	}
}

// ── CRUD + masking ──────────────────────────────────────────────────

func TestWS06_CreateGetListMasked(t *testing.T) {
	seg := ws06Seg("crud")
	p := ws06Provider(t, seg)
	token := "sk-ws06-" + seg + "-SECRETSECRET-9z8y"
	base := FX(t).URLs.FakeLLM + "/ws06/" + seg + "-ovr/openai/v1"
	id := ws06Key(t, map[string]any{
		"provider_id": p, "key_type": "api_key", "token": token, "base_url": base,
		"name": "ws06-crud ✨ ключ", "description": "primary key",
	})
	wantMask := token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]

	r := AdminAPI(t).Get(t, "/provider-keys/"+id)
	ws06Expect(t, r, http.StatusOK, "", "")
	ws06AssertNoSecret(t, r.Body, token, token[4:len(token)-4])
	k := r.Map(t)
	want := map[string]any{
		"id": id, "provider_id": p, "key_type": "api_key", "token_masked": wantMask,
		"base_url": base, "name": "ws06-crud ✨ ключ", "description": "primary key", "status": "active",
	}
	for f, v := range want {
		if k[f] != v {
			t.Errorf("detail %s = %v, want %v", f, k[f], v)
		}
	}
	// Exact field set: nothing beyond the documented public fields.
	allowed := map[string]bool{"id": true, "provider_id": true, "key_type": true, "token_masked": true, "base_url": true,
		"name": true, "description": true, "status": true, "sort_order": true, "created_at": true, "updated_at": true}
	for f := range k {
		if !allowed[f] {
			t.Errorf("detail exposes unexpected field %q", f)
		}
	}

	// List (filtered by provider) returns the same masked form, never the secret.
	lr := AdminAPI(t).Get(t, "/provider-keys?provider_id="+p)
	ws06Expect(t, lr, http.StatusOK, "", "")
	ws06AssertNoSecret(t, lr.Body, token)
	items := ws06List(t, "provider_id="+p)
	if len(items) != 1 || items[0]["id"] != id || items[0]["token_masked"] != wantMask {
		t.Fatalf("list by provider: %v", items)
	}
	// Unfiltered list also hides secrets (contains seeded keys too).
	all := AdminAPI(t).Get(t, "/provider-keys?limit=100")
	ws06AssertNoSecret(t, all.Body, token, "sk-e2e-openai", "sk-ant-e2e", "AIza-e2e")
}

func TestWS06_MaskShortTokensFullyHidden(t *testing.T) {
	p := ws06Provider(t, ws06Seg("short"))
	for _, tok := range []string{"a", "abcd", "abcdefgh"} {
		id := ws06APIKey(t, p, "ws06-short-"+tok, tok)
		if got := ws06Get(t, id)["token_masked"]; got != strings.Repeat("*", len(tok)) {
			t.Errorf("token %q masked as %q, want all stars", tok, got)
		}
	}
}

// FINDING WS06-1: for 9–15 character tokens the mask reveals 8 characters —
// e.g. a 9-char token shows 8 of its 9 characters in list/detail.
func TestWS06_MaskDoesNotRevealMostOfShortToken(t *testing.T) {
	p := ws06Provider(t, ws06Seg("mask9"))
	tok := "Zq7Kx2Pw9"
	id := ws06APIKey(t, p, "ws06-mask9", tok)
	masked := ws06Get(t, id)["token_masked"].(string)
	revealed := len(masked) - strings.Count(masked, "*")
	if revealed*2 > len(tok) {
		t.Fatalf("token_masked %q reveals %d of %d secret characters (want at most half)", masked, revealed, len(tok))
	}
}

func TestWS06_UpdateFieldsAndStatus(t *testing.T) {
	seg := ws06Seg("upd")
	p := ws06Provider(t, seg)
	token := "sk-ws06-update-original-0001"
	id := ws06APIKey(t, p, "ws06-upd", token)
	admin := AdminAPI(t)
	before := ws06Get(t, id)

	r := admin.Put(t, "/provider-keys/"+id, map[string]any{
		"name": "ws06-upd-renamed", "description": "new desc", "status": "inactive", "sort_order": 3,
		"base_url": "http://localhost:29100/ws06/" + seg + "-x/openai/v1",
	})
	ws06Expect(t, r, http.StatusNoContent, "", "")
	if len(r.Body) != 0 {
		t.Fatalf("update response must be empty, got %s", r.Body)
	}
	k := ws06Get(t, id)
	for f, v := range map[string]any{"name": "ws06-upd-renamed", "description": "new desc", "status": "inactive",
		"sort_order": float64(3), "base_url": "http://localhost:29100/ws06/" + seg + "-x/openai/v1",
		"token_masked": before["token_masked"], "key_type": "api_key", "created_at": before["created_at"]} {
		if k[f] != v {
			t.Errorf("%s = %v, want %v", f, k[f], v)
		}
	}
	if k["updated_at"] == before["updated_at"] {
		t.Errorf("updated_at did not change")
	}

	// Status filter.
	inactive := ws06List(t, "provider_id="+p+"&status=inactive")
	active := ws06List(t, "provider_id="+p+"&status=active")
	if len(inactive) != 1 || len(active) != 0 {
		t.Fatalf("status filter: inactive=%d active=%d", len(inactive), len(active))
	}

	// Empty body is a no-op.
	ws06Expect(t, admin.Put(t, "/provider-keys/"+id, map[string]any{}), http.StatusNoContent, "", "")
	if ws06Get(t, id)["name"] != "ws06-upd-renamed" {
		t.Fatal("empty update changed the name")
	}
	// Reactivate.
	ws06Expect(t, admin.Put(t, "/provider-keys/"+id, map[string]any{"status": "active"}), http.StatusNoContent, "", "")
	if ws06Get(t, id)["status"] != "active" {
		t.Fatal("reactivate failed")
	}
}

func TestWS06_RotateTokenChangesMaskAndCiphertext(t *testing.T) {
	p := ws06Provider(t, ws06Seg("rot"))
	id := ws06APIKey(t, p, "ws06-rot", "sk-ws06-rotate-old-AAAA1111")
	row1 := ws06Stored(t, id)
	newTok := "sk-ws06-rotate-new-BBBB2222"
	r := AdminAPI(t).Put(t, "/provider-keys/"+id, map[string]any{"token": newTok})
	ws06Expect(t, r, http.StatusNoContent, "", "")
	ws06AssertNoSecret(t, r.Body, newTok)
	k := ws06Get(t, id)
	if k["token_masked"] != "sk-w*******************2222" {
		t.Fatalf("masked after rotate = %v", k["token_masked"])
	}
	row2 := ws06Stored(t, id)
	if row2.Cipher == row1.Cipher || row2.Hash == row1.Hash {
		t.Fatalf("rotation did not change stored ciphertext/hash")
	}
	ws06AssertOpaque(t, row2.Cipher, newTok, "sk-ws06-rotate-old-AAAA1111")
}

func TestWS06_DeleteKey(t *testing.T) {
	p := ws06Provider(t, ws06Seg("del"))
	id := ws06APIKey(t, p, "ws06-del", "sk-ws06-delete-me-1234")
	admin := AdminAPI(t)
	ws06Expect(t, admin.Delete(t, "/provider-keys/"+id), http.StatusNoContent, "", "")
	ws06Expect(t, admin.Get(t, "/provider-keys/"+id), http.StatusNotFound, "NOT_FOUND", "provider key not found")
	ws06Expect(t, admin.Delete(t, "/provider-keys/"+id), http.StatusNotFound, "NOT_FOUND", "provider key not found")
	ws06Expect(t, admin.Put(t, "/provider-keys/"+id, map[string]any{"name": "x"}), http.StatusNotFound, "NOT_FOUND", "provider key not found")
	if n := len(ws06List(t, "provider_id="+p)); n != 0 {
		t.Fatalf("deleted key still listed (%d)", n)
	}
	var cnt int
	if err := ws06Postgres(t).QueryRow(`SELECT count(*) FROM provider_keys WHERE id=$1`, id).Scan(&cnt); err != nil || cnt != 0 {
		t.Fatalf("row still in DB: count=%d err=%v", cnt, err)
	}
}

// ── validation ──────────────────────────────────────────────────────

func TestWS06_CreateValidation(t *testing.T) {
	p := ws06Provider(t, ws06Seg("val"))
	admin := AdminAPI(t)
	exp := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	cases := []struct {
		name   string
		body   any
		status int
		code   string
		msg    string
	}{
		{"empty object", map[string]any{}, 400, "VALIDATION", "provider_id is required"},
		{"missing token", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "api_key"}, 400, "VALIDATION", "token is required for api_key keys"},
		{"blank token", map[string]any{"provider_id": p, "name": "ws06-x", "token": "   "}, 400, "VALIDATION", "token is required for api_key keys"},
		{"empty name", map[string]any{"provider_id": p, "name": "", "token": "sk-x"}, 400, "VALIDATION", "name is required"},
		{"blank name", map[string]any{"provider_id": p, "name": " \t ", "token": "sk-x"}, 400, "VALIDATION", "name is required"},
		{"unknown key_type", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "bearer", "token": "sk-x"}, 400, "VALIDATION", "invalid key_type: must be api_key or oauth"},
		{"malformed provider_id", map[string]any{"provider_id": "not-a-uuid", "name": "ws06-x", "token": "sk-x"}, 400, "VALIDATION", ""},
		{"unknown provider_id", map[string]any{"provider_id": ws06Missing, "name": "ws06-x", "token": "sk-x"}, 404, "NOT_FOUND", "provider not found"},
		{"oauth without oauth_data", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "oauth", "token": "sk-x"}, 400, "VALIDATION", "oauth_data is required for oauth keys"},
		{"oauth missing access", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "oauth", "oauth_data": map[string]any{"refresh_token": "r", "expires_at": exp}}, 400, "VALIDATION", "oauth_data.access_token is required"},
		{"oauth missing refresh", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "oauth", "oauth_data": map[string]any{"access_token": "a", "expires_at": exp}}, 400, "VALIDATION", "oauth_data.refresh_token is required"},
		{"oauth missing expiry", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "oauth", "oauth_data": map[string]any{"access_token": "a", "refresh_token": "r"}}, 400, "VALIDATION", "oauth_data.expires_at is required"},
		{"oauth bad expiry", map[string]any{"provider_id": p, "name": "ws06-x", "key_type": "oauth", "oauth_data": map[string]any{"access_token": "a", "refresh_token": "r", "expires_at": "tomorrow"}}, 400, "VALIDATION", "invalid request body"},
		{"not json", "{nope", 400, "VALIDATION", "invalid request body"},
		{"token wrong type", map[string]any{"provider_id": p, "name": "ws06-x", "token": 123}, 400, "VALIDATION", "invalid request body"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r Resp
			if s, ok := c.body.(string); ok {
				r = admin.Do(t, http.MethodPost, "/provider-keys", []byte(s))
			} else {
				r = admin.Post(t, "/provider-keys", c.body)
			}
			ws06Expect(t, r, c.status, c.code, c.msg)
		})
	}
	if n := len(ws06List(t, "provider_id="+p)); n != 0 {
		t.Fatalf("rejected creates left %d keys behind", n)
	}
}

// FINDING WS06-2: a key's base_url override is stored verbatim with no URL
// validation (the gateway then sends live credentials to whatever it says).
func TestWS06_CreateRejectsInvalidBaseURL(t *testing.T) {
	p := ws06Provider(t, ws06Seg("badurl"))
	admin := AdminAPI(t)
	for _, u := range []string{"not a url", "ftp://localhost/x", "javascript:alert(1)", "localhost:29100/no-scheme"} {
		t.Run(u, func(t *testing.T) {
			r := admin.Post(t, "/provider-keys", map[string]any{"provider_id": p, "name": "ws06-badurl", "token": "sk-ws06-badurl-1", "base_url": u})
			if r.Status == http.StatusCreated {
				id := r.Map(t)["id"].(string)
				admin.Delete(t, "/provider-keys/"+id)
			}
			ws06Expect(t, r, http.StatusBadRequest, "VALIDATION", "")
		})
	}
}

// FINDING WS06-3: a token with non-ASCII characters near the start/end makes the
// byte-sliced mask invalid UTF-8 → 500 "database error" instead of 201 (or 400).
func TestWS06_CreateUnicodeToken(t *testing.T) {
	p := ws06Provider(t, ws06Seg("uni"))
	r := AdminAPI(t).Post(t, "/provider-keys", map[string]any{"provider_id": p, "name": "ws06-uni", "token": "abcé-secret-tokené"})
	if r.Status == http.StatusCreated {
		id := r.Map(t)["id"].(string)
		t.Cleanup(func() { AdminAPI(t).Delete(t, "/provider-keys/"+id) })
	}
	if r.Status != http.StatusCreated && r.Status != http.StatusBadRequest {
		t.Fatalf("non-ASCII token: want 201 (or a 400 validation error), got %d %s", r.Status, r.Body)
	}
}

func TestWS06_UpdateValidation(t *testing.T) {
	p := ws06Provider(t, ws06Seg("uval"))
	api := ws06APIKey(t, p, "ws06-uval-api", "sk-ws06-uval-api-0001")
	oauth := ws06Key(t, map[string]any{"provider_id": p, "name": "ws06-uval-oauth", "key_type": "oauth",
		"oauth_data": map[string]any{"access_token": "acc-ws06-uval-0001", "refresh_token": "ref-ws06-uval", "expires_at": "2031-01-01T00:00:00Z"}})
	admin := AdminAPI(t)
	od := map[string]any{"access_token": "a2", "refresh_token": "r2", "expires_at": "2031-01-01T00:00:00Z"}
	cases := []struct {
		name, id string
		body     any
		status   int
		code     string
		msg      string
	}{
		{"empty name", api, map[string]any{"name": ""}, 400, "VALIDATION", "name cannot be empty"},
		{"blank token", api, map[string]any{"token": "  "}, 400, "VALIDATION", "token cannot be empty"},
		{"bad status", api, map[string]any{"status": "disabled"}, 400, "VALIDATION", "invalid key status"},
		{"token and oauth", api, map[string]any{"token": "x", "oauth_data": od}, 400, "VALIDATION", "cannot set both token and oauth_data"},
		{"oauth_data on api_key", api, map[string]any{"oauth_data": od}, 400, "VALIDATION", "cannot set oauth_data on an api_key key; use token"},
		{"token on oauth", oauth, map[string]any{"token": "x"}, 400, "VALIDATION", "cannot set token on an oauth key; use oauth_data"},
		{"oauth blank access", oauth, map[string]any{"oauth_data": map[string]any{"access_token": "", "refresh_token": "r", "expires_at": "2031-01-01T00:00:00Z"}}, 400, "VALIDATION", "oauth_data.access_token cannot be empty"},
		{"bad id", "nope", map[string]any{"name": "x"}, 400, "VALIDATION", "provider_key_id must be a valid UUID"},
		{"missing id", ws06Missing, map[string]any{"name": "x"}, 404, "NOT_FOUND", "provider key not found"},
		{"not json", api, "{", 400, "VALIDATION", "invalid request body"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r Resp
			if s, ok := c.body.(string); ok {
				r = admin.Do(t, http.MethodPut, "/provider-keys/"+c.id, []byte(s))
			} else {
				r = admin.Put(t, "/provider-keys/"+c.id, c.body)
			}
			ws06Expect(t, r, c.status, c.code, c.msg)
		})
	}
	// Nothing changed.
	if k := ws06Get(t, api); k["name"] != "ws06-uval-api" || k["token_masked"] != "sk-w*************0001" || k["status"] != "active" {
		t.Fatalf("api key mutated by rejected updates: %v", k)
	}
	if k := ws06Get(t, oauth); k["token_masked"] != "oauth:****0001" {
		t.Fatalf("oauth key mutated by rejected updates: %v", k)
	}
}

func TestWS06_ListValidationAndAuth(t *testing.T) {
	f := FX(t)
	admin := AdminAPI(t)
	ws06Expect(t, admin.Get(t, "/provider-keys?provider_id=zzz"), 400, "VALIDATION", "provider_id must be a valid UUID")
	ws06Expect(t, admin.Get(t, "/provider-keys/not-a-uuid"), 400, "VALIDATION", "provider_key_id must be a valid UUID")
	ws06Expect(t, admin.Get(t, "/provider-keys/"+ws06Missing), 404, "NOT_FOUND", "provider key not found")
	ws06Expect(t, admin.Delete(t, "/provider-keys/not-a-uuid"), 400, "VALIDATION", "provider_key_id must be a valid UUID")

	anon := Anon(f.URLs.API)
	if r := anon.Get(t, "/provider-keys"); r.Status != http.StatusUnauthorized {
		t.Fatalf("anon list: %d", r.Status)
	}
	if r := anon.Post(t, "/provider-keys", map[string]any{}); r.Status != http.StatusUnauthorized {
		t.Fatalf("anon create: %d", r.Status)
	}
	np := Bearer(f.URLs.API, f.Personas["noperm_key"].Secret)
	for _, r := range []Resp{
		np.Get(t, "/provider-keys"),
		np.Get(t, "/provider-keys/"+f.ProviderKeys["openai"]),
		np.Post(t, "/provider-keys", map[string]any{"provider_id": f.Providers["openai"], "name": "ws06-np", "token": "x"}),
		np.Put(t, "/provider-keys/"+f.ProviderKeys["openai"], map[string]any{"name": "ws06-np"}),
		np.Delete(t, "/provider-keys/"+f.ProviderKeys["openai"]),
	} {
		if r.Status != http.StatusForbidden {
			t.Fatalf("noperm_key: want 403, got %d %s", r.Status, r.Body)
		}
	}
	gw := Bearer(f.URLs.API, f.Personas["gw_key"].Secret)
	if r := gw.Get(t, "/provider-keys"); r.Status != http.StatusForbidden {
		t.Fatalf("gw_key list keys: want 403, got %d", r.Status)
	}
	// Seeded key untouched.
	if k := ws06Get(t, f.ProviderKeys["openai"]); k["name"] != "e2e-openai-key" || k["status"] != "active" {
		t.Fatalf("seeded key changed: %v", k)
	}
}

func TestWS06_PaginationAndKeyTypeFilter(t *testing.T) {
	p := ws06Provider(t, ws06Seg("page"))
	for i := 0; i < 5; i++ {
		ws06APIKey(t, p, fmt.Sprintf("ws06-page-%d", i), fmt.Sprintf("sk-ws06-page-%04d", i))
	}
	ws06Key(t, map[string]any{"provider_id": p, "name": "ws06-page-oauth", "key_type": "oauth",
		"oauth_data": map[string]any{"access_token": "acc-ws06-page-XYZ9", "refresh_token": "r", "expires_at": "2031-01-01T00:00:00Z"}})

	r := AdminAPI(t).Get(t, "/provider-keys?provider_id="+p+"&limit=2&offset=2")
	var out struct {
		Items []ws06KeyView
		Page  struct{ Total, Limit, Offset int }
	}
	r.JSON(t, &out)
	if out.Page.Total != 6 || out.Page.Limit != 2 || out.Page.Offset != 2 || len(out.Items) != 2 {
		t.Fatalf("page: %+v items=%d", out.Page, len(out.Items))
	}
	// Ordered by created_at (no sort_order): page 2 = keys 2 and 3.
	if out.Items[0]["name"] != "ws06-page-2" || out.Items[1]["name"] != "ws06-page-3" {
		t.Fatalf("order: %v, %v", out.Items[0]["name"], out.Items[1]["name"])
	}
	oauth := ws06List(t, "provider_id="+p+"&key_type=oauth")
	if len(oauth) != 1 || oauth[0]["name"] != "ws06-page-oauth" {
		t.Fatalf("key_type=oauth filter: %v", oauth)
	}
	if n := len(ws06List(t, "provider_id="+p+"&key_type=api_key")); n != 5 {
		t.Fatalf("key_type=api_key filter: %d", n)
	}
}

func TestWS06_ConcurrentCreates(t *testing.T) {
	p := ws06Provider(t, ws06Seg("conc"))
	admin := AdminAPI(t)
	const n = 10
	ids := make([]string, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := admin.Post(t, "/provider-keys", map[string]any{"provider_id": p, "name": fmt.Sprintf("ws06-conc-%d", i), "token": fmt.Sprintf("sk-ws06-conc-%04d", i)})
			codes[i] = r.Status
			if r.Status == http.StatusCreated {
				var o struct{ ID string }
				_ = json.Unmarshal(r.Body, &o)
				ids[i] = o.ID
			}
		}(i)
	}
	wg.Wait()
	for i, id := range ids {
		if id != "" {
			id := id
			t.Cleanup(func() { admin.Delete(t, "/provider-keys/"+id) })
		}
		if codes[i] != http.StatusCreated {
			t.Errorf("create %d: %d", i, codes[i])
		}
	}
	if got := len(ws06List(t, "provider_id="+p+"&limit=50")); got != n {
		t.Fatalf("want %d keys, got %d", n, got)
	}
}

// Duplicate tokens on the same provider are accepted (token_hash is not unique).
func TestWS06_DuplicateTokenAllowed(t *testing.T) {
	p := ws06Provider(t, ws06Seg("dup"))
	a := ws06APIKey(t, p, "ws06-dup", "sk-ws06-duplicate-0001")
	b := ws06APIKey(t, p, "ws06-dup", "sk-ws06-duplicate-0001")
	ra, rb := ws06Stored(t, a), ws06Stored(t, b)
	if ra.Cipher == rb.Cipher {
		t.Fatal("same plaintext produced identical ciphertext (nonce reuse?)")
	}
	if ra.Hash != rb.Hash {
		t.Fatal("same plaintext produced different token_hash")
	}
}

func TestWS06_LongValues(t *testing.T) {
	p := ws06Provider(t, ws06Seg("long"))
	name := "ws06-" + strings.Repeat("n", 995)
	token := "sk-" + strings.Repeat("x", 8000) + "TAIL"
	id := ws06APIKey(t, p, name, token)
	k := ws06Get(t, id)
	if k["name"] != name {
		t.Fatal("long name not round-tripped")
	}
	if k["token_masked"] != "sk-x"+strings.Repeat("*", len(token)-8)+"TAIL" {
		t.Fatalf("long token mask wrong (len %d)", len(k["token_masked"].(string)))
	}
}

// ── encryption at rest ──────────────────────────────────────────────

func TestWS06_EncryptionAtRest(t *testing.T) {
	p := ws06Provider(t, ws06Seg("enc"))
	token := "sk-ws06-encrypt-PLAINTEXT-7f3a9c"
	id := ws06APIKey(t, p, "ws06-enc", token)
	row := ws06Stored(t, id)
	if row.Cipher == token || row.Hash == token {
		t.Fatal("token stored in plaintext")
	}
	ws06AssertOpaque(t, row.Cipher, token, "PLAINTEXT-7f3a9c")
	ws06AssertOpaque(t, row.Hash, token)
	if row.Masked != token[:4]+strings.Repeat("*", len(token)-8)+token[len(token)-4:] {
		t.Fatalf("token_masked column = %q", row.Masked)
	}
	if len(row.Hash) != 64 {
		t.Fatalf("token_hash should be a 64-hex HMAC-SHA256, got %q", row.Hash)
	}
	if _, err := hex.DecodeString(row.Hash); err != nil {
		t.Fatalf("token_hash not hex: %v", err)
	}
	// No other column of the row contains the secret either.
	var dump string
	if err := ws06Postgres(t).QueryRow(`SELECT row_to_json(k)::text FROM provider_keys k WHERE id=$1`, id).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, token) {
		t.Fatalf("row contains plaintext: %s", dump)
	}

	oid := ws06Key(t, map[string]any{"provider_id": p, "name": "ws06-enc-oauth", "key_type": "oauth",
		"oauth_data": map[string]any{"access_token": "acc-ws06-enc-ACCESS-55aa", "refresh_token": "ref-ws06-enc-REFRESH-66bb", "expires_at": "2031-01-01T00:00:00Z"}})
	orow := ws06Stored(t, oid)
	if orow.KeyType != "oauth" {
		t.Fatalf("key_type column = %q", orow.KeyType)
	}
	ws06AssertOpaque(t, orow.Cipher, "acc-ws06-enc-ACCESS-55aa", "ref-ws06-enc-REFRESH-66bb", "access_token", "refresh_token")
	if err := ws06Postgres(t).QueryRow(`SELECT row_to_json(k)::text FROM provider_keys k WHERE id=$1`, oid).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, "ACCESS-55aa") || strings.Contains(dump, "REFRESH-66bb") {
		t.Fatalf("oauth row contains plaintext: %s", dump)
	}
}

// ── OAuth keys ──────────────────────────────────────────────────────

func TestWS06_OAuthKeyCreateAndRotate(t *testing.T) {
	seg := ws06Seg("oauth")
	p := ws06Provider(t, seg)
	access, refresh := "acc-ws06-oauth-first-ABCD", "ref-ws06-oauth-first"
	id := ws06Key(t, map[string]any{"provider_id": p, "name": "ws06-oauth", "key_type": "oauth",
		"oauth_data": map[string]any{"access_token": access, "refresh_token": refresh, "expires_at": "2031-06-01T00:00:00Z"}})
	r := AdminAPI(t).Get(t, "/provider-keys/"+id)
	ws06AssertNoSecret(t, r.Body, access, refresh)
	k := r.Map(t)
	if k["key_type"] != "oauth" || k["token_masked"] != "oauth:****ABCD" || k["status"] != "active" {
		t.Fatalf("oauth key view: %v", k)
	}

	// Gateway sends the access token as Bearer for an oauth key on an openai provider.
	model := ws06Model(t, p, seg)
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+access {
		t.Fatalf("upstream Authorization = %q, want Bearer <access_token>", auth)
	}

	// Replace OAuth tokens.
	access2 := "acc-ws06-oauth-second-WXYZ"
	u := AdminAPI(t).Put(t, "/provider-keys/"+id, map[string]any{"oauth_data": map[string]any{
		"access_token": access2, "refresh_token": "ref-ws06-oauth-second", "expires_at": "2031-07-01T00:00:00Z"}})
	ws06Expect(t, u, http.StatusNoContent, "", "")
	if m := ws06Get(t, id)["token_masked"]; m != "oauth:****WXYZ" {
		t.Fatalf("masked after oauth rotate = %v", m)
	}
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+access2 {
		t.Fatalf("after oauth rotate upstream Authorization = %q", auth)
	}
}

// Documents the refresh worker against the e2e stack: refreshers exist only for
// the claude-code and codex protocols, and their token URLs are hard-coded to
// platform.claude.com / auth.openai.com, so a real refresh cannot be driven
// against fakellm. What is observable: an expired oauth key keeps being used
// as-is by the gateway (no pre-flight refresh), and the 1-minute background
// worker picks it up and logs a refresh failure (no refresher for "openai")
// without disabling or altering the key.
func TestWS06_OAuthExpiredKeyRefreshWorker(t *testing.T) {
	seg := ws06Seg("oexp")
	p := ws06Provider(t, seg)
	access := "acc-ws06-expired-" + seg
	id := ws06Key(t, map[string]any{"provider_id": p, "name": "ws06-oexp", "key_type": "oauth",
		"oauth_data": map[string]any{"access_token": access, "refresh_token": "ref-ws06-expired", "expires_at": "2020-01-01T00:00:00Z"}})
	model := ws06Model(t, p, seg)
	before := ws06Stored(t, id)

	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+access {
		t.Fatalf("expired oauth key: upstream saw %q", auth)
	}

	_, file, _, _ := runtime.Caller(0)
	logPath := filepath.Join(filepath.Dir(file), "..", ".run", "server.log")
	want := "oauth refresh: failed to refresh key " + id
	Eventually(t, 75*time.Second, func() bool {
		b, err := os.ReadFile(logPath)
		return err == nil && strings.Contains(string(b), want)
	}, "refresh worker did not attempt the expired key ("+want+")")
	b, _ := os.ReadFile(logPath)
	if !strings.Contains(string(b), "no token refresher registered for provider protocol openai") {
		t.Fatalf("refresh failure reason not logged as missing refresher")
	}
	k := ws06Get(t, id)
	if k["status"] != "active" || ws06Stored(t, id).Cipher != before.Cipher {
		t.Fatalf("failed refresh mutated key: %v", k)
	}
}

// ── gateway integration ─────────────────────────────────────────────

func TestWS06_GatewayDisabledKeySkippedAndDeleteMakesUnroutable(t *testing.T) {
	seg := ws06Seg("gw")
	p := ws06Provider(t, seg)
	tokA, tokB := "sk-ws06-gw-AAAA-"+seg, "sk-ws06-gw-BBBB-"+seg
	a := ws06APIKey(t, p, "ws06-gw-A", tokA)
	time.Sleep(20 * time.Millisecond) // distinct created_at → A is ordered first
	b := ws06APIKey(t, p, "ws06-gw-B", tokB)
	model := ws06Model(t, p, seg)
	admin := AdminAPI(t)

	// Both active and healthy: first by created_at (A).
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+tokA {
		t.Fatalf("both active: upstream saw %q, want key A", auth)
	}
	// Disable A → B is used.
	ws06Expect(t, admin.Put(t, "/provider-keys/"+a, map[string]any{"status": "inactive"}), http.StatusNoContent, "", "")
	for i := 0; i < 2; i++ {
		if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+tokB {
			t.Fatalf("A disabled: upstream saw %q, want key B", auth)
		}
	}
	// Disable B too → no active key → unroutable, nothing sent upstream.
	ws06Expect(t, admin.Put(t, "/provider-keys/"+b, map[string]any{"status": "inactive"}), http.StatusNoContent, "", "")
	before := len(ws06Upstream(t, seg))
	ws06Expect(t, ws06Chat(t, model), http.StatusNotFound, "NOT_FOUND", "no available route for model: "+model)
	if len(ws06Upstream(t, seg)) != before {
		t.Fatal("upstream called although every key is inactive")
	}
	// Re-enable A → A again.
	ws06Expect(t, admin.Put(t, "/provider-keys/"+a, map[string]any{"status": "active"}), http.StatusNoContent, "", "")
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+tokA {
		t.Fatalf("A re-enabled: upstream saw %q", auth)
	}
	// Delete A (B still inactive) → unroutable.
	ws06Expect(t, admin.Delete(t, "/provider-keys/"+a), http.StatusNoContent, "", "")
	before = len(ws06Upstream(t, seg))
	ws06Expect(t, ws06Chat(t, model), http.StatusNotFound, "NOT_FOUND", "no available route for model: "+model)
	if len(ws06Upstream(t, seg)) != before {
		t.Fatal("upstream called after the only active key was deleted")
	}
	// Re-activate B → routable with B.
	ws06Expect(t, admin.Put(t, "/provider-keys/"+b, map[string]any{"status": "active"}), http.StatusNoContent, "", "")
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+tokB {
		t.Fatalf("B re-activated: upstream saw %q", auth)
	}
	// Delete B → unroutable.
	ws06Expect(t, admin.Delete(t, "/provider-keys/"+b), http.StatusNoContent, "", "")
	ws06Expect(t, ws06Chat(t, model), http.StatusNotFound, "NOT_FOUND", "no available route for model: "+model)
}

func TestWS06_GatewayRotationChangesAuthorization(t *testing.T) {
	seg := ws06Seg("gwrot")
	p := ws06Provider(t, seg)
	old := "sk-ws06-rot-OLD-" + seg
	id := ws06APIKey(t, p, "ws06-gwrot", old)
	model := ws06Model(t, p, seg)
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+old {
		t.Fatalf("before rotate: %q", auth)
	}
	nw := "sk-ws06-rot-NEW-" + seg
	ws06Expect(t, AdminAPI(t).Put(t, "/provider-keys/"+id, map[string]any{"token": nw}), http.StatusNoContent, "", "")
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+nw {
		t.Fatalf("after rotate upstream saw %q, want new token", auth)
	}
}

func TestWS06_GatewayKeyBaseURLOverride(t *testing.T) {
	seg := ws06Seg("gwurl")
	p := ws06Provider(t, seg)
	tok := "sk-ws06-url-" + seg
	id := ws06Key(t, map[string]any{"provider_id": p, "name": "ws06-gwurl", "token": tok,
		"base_url": FX(t).URLs.FakeLLM + "/ws06/" + seg + "/override/v1"})
	model := ws06Model(t, p, seg)
	auth, path := ws06ChatAuth(t, model, seg)
	if path != "/ws06/"+seg+"/override/v1/chat/completions" || auth != "Bearer "+tok {
		t.Fatalf("override: path=%q auth=%q", path, auth)
	}
	// Clearing the override (empty string) falls back to the provider base URL.
	ws06Expect(t, AdminAPI(t).Put(t, "/provider-keys/"+id, map[string]any{"base_url": ""}), http.StatusNoContent, "", "")
	if _, path = ws06ChatAuth(t, model, seg); path != "/ws06/"+seg+"/openai/v1/chat/completions" {
		t.Fatalf("after clearing override path=%q", path)
	}
}

func TestWS06_GatewaySortOrderPicksKey(t *testing.T) {
	seg := ws06Seg("gwsort")
	p := ws06Provider(t, seg)
	tokA, tokB := "sk-ws06-sort-A-"+seg, "sk-ws06-sort-B-"+seg
	ws06APIKey(t, p, "ws06-sort-A", tokA)
	time.Sleep(20 * time.Millisecond)
	b := ws06APIKey(t, p, "ws06-sort-B", tokB)
	model := ws06Model(t, p, seg)
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+tokA {
		t.Fatalf("default order: %q", auth)
	}
	ws06Expect(t, AdminAPI(t).Put(t, "/provider-keys/"+b, map[string]any{"sort_order": 0}), http.StatusNoContent, "", "")
	if auth, _ := ws06ChatAuth(t, model, seg); auth != "Bearer "+tokB {
		t.Fatalf("sort_order=0 on B: upstream saw %q, want B", auth)
	}
}
