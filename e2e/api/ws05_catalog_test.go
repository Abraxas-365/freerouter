//go:build e2e

package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// WS-05: catalog — providers, models, mappings, fallbacks.
// Everything created here is prefixed ws05- and deleted in t.Cleanup. Gateway
// calls only hit providers pointed at fakellm under /ws05*/ path prefixes.

// ── helpers ─────────────────────────────────────────────────────────

const ws05Missing = "00000000-0000-4000-8000-000000000005"

func ws05Expect(t *testing.T, r Resp, status int, code, msg string) {
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

func ws05ID(t *testing.T, r Resp) string {
	t.Helper()
	if r.Status != http.StatusCreated {
		t.Fatalf("want 201, got %d %s", r.Status, r.Body)
	}
	var out struct{ ID string }
	r.JSON(t, &out)
	if out.ID == "" {
		t.Fatalf("no id in %s", r.Body)
	}
	return out.ID
}

// ws05Provider creates a provider and registers its deletion (404 on cleanup is fine).
func ws05Provider(t *testing.T, body map[string]any) string {
	t.Helper()
	if _, ok := body["protocol"]; !ok {
		body["protocol"] = "openai"
	}
	if _, ok := body["base_url"]; !ok {
		body["base_url"] = "http://localhost:29100/ws05/openai/v1"
	}
	id := ws05ID(t, AdminAPI(t).Post(t, "/providers", body))
	t.Cleanup(func() { AdminAPI(t).Delete(t, "/providers/"+id) })
	return id
}

func ws05Model(t *testing.T, name string) string {
	t.Helper()
	id := ws05ID(t, AdminAPI(t).Post(t, "/models", map[string]any{"name": name, "family": "ws05", "description": "ws05 test"}))
	t.Cleanup(func() { AdminAPI(t).Delete(t, "/models/"+id) })
	return id
}

func ws05Mapping(t *testing.T, body map[string]any) string {
	t.Helper()
	id := ws05ID(t, AdminAPI(t).Post(t, "/mappings", body))
	t.Cleanup(func() { AdminAPI(t).Delete(t, "/mappings/"+id) })
	return id
}

func ws05Key(t *testing.T, provider string) string {
	t.Helper()
	id := ws05ID(t, AdminAPI(t).Post(t, "/provider-keys", map[string]any{
		"provider_id": provider, "name": Uniq("ws05-key"), "key_type": "api_key", "token": "sk-ws05-fake",
	}))
	t.Cleanup(func() { AdminAPI(t).Delete(t, "/provider-keys/"+id) })
	return id
}

// ws05Routable builds provider(+key) → model → mapping pointed at fakellm under /<prefix>/.
func ws05Routable(t *testing.T, prefix string) (provider string) {
	t.Helper()
	p := ws05Provider(t, map[string]any{
		"name": Uniq("ws05-" + prefix), "base_url": "http://localhost:29100/" + prefix + "/openai/v1",
	})
	ws05Key(t, p)
	return p
}

func ws05UpstreamModels(t *testing.T, prefix string) []string {
	t.Helper()
	var out []string
	for _, r := range FakeLLMRequests(t) {
		if p, _ := r["path"].(string); strings.HasPrefix(p, "/"+prefix+"/") {
			m, _ := r["model"].(string)
			out = append(out, m)
		}
	}
	return out
}

func ws05List(t *testing.T, path string) (items []map[string]any, total int) {
	t.Helper()
	r := AdminAPI(t).Get(t, path)
	if r.Status != 200 {
		t.Fatalf("GET %s: %d %s", path, r.Status, r.Body)
	}
	var out struct {
		Items []map[string]any
		Page  struct{ Total int }
	}
	r.JSON(t, &out)
	return out.Items, out.Page.Total
}

func ws05Fallbacks(t *testing.T, model string) []map[string]any {
	t.Helper()
	r := AdminAPI(t).Get(t, "/model-fallbacks/by-model/"+model)
	if r.Status != 200 {
		t.Fatalf("list fallbacks: %d %s", r.Status, r.Body)
	}
	var out []map[string]any
	r.JSON(t, &out)
	return out
}

func ws05GatewayModelIDs(t *testing.T) []string {
	t.Helper()
	f := FX(t)
	r := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret).Get(t, "/models")
	if r.Status != 200 {
		t.Fatalf("/v1/models: %d %s", r.Status, r.Body)
	}
	var out struct {
		Object string
		Data   []struct{ ID, Object, OwnedBy string }
	}
	r.JSON(t, &out)
	if out.Object != "list" {
		t.Fatalf("/v1/models object=%q", out.Object)
	}
	ids := make([]string, 0, len(out.Data))
	for _, d := range out.Data {
		ids = append(ids, d.ID)
	}
	return ids
}

func ws05Count(xs []string, v string) int {
	n := 0
	for _, x := range xs {
		if x == v {
			n++
		}
	}
	return n
}

// ── providers ───────────────────────────────────────────────────────

func TestWS05_ProviderCRUD(t *testing.T) {
	api := AdminAPI(t)
	name := Uniq("ws05-prov") + " ünïcødé 🚀"
	id := ws05Provider(t, map[string]any{
		"name": name, "protocol": "anthropic", "description": "ws05 desc",
		"website": "https://example.com", "base_url": "http://localhost:29100/ws05/anthropic", "streaming": true,
	})

	r := api.Get(t, "/providers/"+id)
	ws05Expect(t, r, 200, "", "")
	p := r.Map(t)
	for k, want := range map[string]any{
		"name": name, "protocol": "anthropic", "description": "ws05 desc", "website": "https://example.com",
		"base_url": "http://localhost:29100/ws05/anthropic", "status": "active", "streaming": true,
	} {
		if p[k] != want {
			t.Errorf("%s: want %v, got %v", k, want, p[k])
		}
	}
	created := p["updated_at"]

	// search finds it (ILIKE, partial, case-insensitive)
	items, total := ws05List(t, "/providers?search="+strings.ToUpper(strings.Split(name, " ")[0]))
	if total != 1 || len(items) != 1 || items[0]["id"] != id {
		t.Errorf("search: total=%d items=%v", total, items)
	}
	// status filter
	items, _ = ws05List(t, "/providers?status=inactive&limit=100")
	for _, it := range items {
		if it["id"] == id {
			t.Errorf("active provider listed under status=inactive")
		}
	}

	// partial update keeps untouched fields
	time.Sleep(10 * time.Millisecond)
	ws05Expect(t, api.Put(t, "/providers/"+id, map[string]any{"status": "inactive", "description": "changed"}), 204, "", "")
	p = api.Get(t, "/providers/"+id).Map(t)
	if p["status"] != "inactive" || p["description"] != "changed" || p["name"] != name || p["protocol"] != "anthropic" || p["streaming"] != true {
		t.Errorf("after partial update: %v", p)
	}
	if p["updated_at"] == created {
		t.Errorf("updated_at not bumped")
	}
	items, _ = ws05List(t, "/providers?status=inactive&search="+strings.Split(name, " ")[0])
	if len(items) != 1 {
		t.Errorf("status=inactive filter: %v", items)
	}
	// streaming=false must be applied (pointer bool, not dropped as zero value)
	ws05Expect(t, api.Put(t, "/providers/"+id, map[string]any{"streaming": false, "protocol": "cohere"}), 204, "", "")
	if p = api.Get(t, "/providers/"+id).Map(t); p["streaming"] != false || p["protocol"] != "cohere" {
		t.Errorf("streaming=false/protocol update not applied: %v", p)
	}

	ws05Expect(t, api.Delete(t, "/providers/"+id), 204, "", "")
	ws05Expect(t, api.Get(t, "/providers/"+id), 404, "NOT_FOUND", "provider not found")
	ws05Expect(t, api.Delete(t, "/providers/"+id), 404, "NOT_FOUND", "provider not found")
	ws05Expect(t, api.Put(t, "/providers/"+id, map[string]any{"name": "x"}), 404, "NOT_FOUND", "provider not found")
}

func TestWS05_ProviderValidation(t *testing.T) {
	api := AdminAPI(t)
	const protoMsg = "invalid protocol; must be one of: openai, anthropic, google, azure, cohere, codex, claude-code"
	cases := []struct {
		name string
		body any
		msg  string
	}{
		{"empty name", map[string]any{"name": "", "protocol": "openai", "base_url": "http://x"}, "provider name is required"},
		{"whitespace name", map[string]any{"name": "  \t ", "protocol": "openai", "base_url": "http://x"}, "provider name is required"},
		{"missing base_url", map[string]any{"name": "ws05-x", "protocol": "openai"}, "provider base_url is required"},
		{"whitespace base_url", map[string]any{"name": "ws05-x", "protocol": "openai", "base_url": "   "}, "provider base_url is required"},
		{"unknown protocol", map[string]any{"name": "ws05-x", "protocol": "bogus", "base_url": "http://x"}, protoMsg},
		{"missing protocol", map[string]any{"name": "ws05-x", "base_url": "http://x"}, protoMsg},
		{"protocol wrong case", map[string]any{"name": "ws05-x", "protocol": "OpenAI", "base_url": "http://x"}, protoMsg},
		{"malformed json", `{"name":"ws05-x",`, "invalid request body"},
		{"wrong type", `{"name":123,"protocol":"openai","base_url":"http://x"}`, "invalid request body"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws05Expect(t, api.Post(t, "/providers", c.body), 400, "VALIDATION", c.msg)
		})
	}

	id := ws05Provider(t, map[string]any{"name": Uniq("ws05-pv")})
	upd := []struct {
		name string
		body any
		msg  string
	}{
		{"empty name", map[string]any{"name": " "}, "provider name cannot be empty"},
		{"bad status", map[string]any{"status": "paused"}, "invalid provider status"},
		{"bad protocol", map[string]any{"protocol": "zzz"}, protoMsg},
		{"malformed", `{`, "invalid request body"},
	}
	for _, c := range upd {
		t.Run("update "+c.name, func(t *testing.T) {
			ws05Expect(t, api.Put(t, "/providers/"+id, c.body), 400, "VALIDATION", c.msg)
		})
	}
	t.Run("bad id", func(t *testing.T) {
		ws05Expect(t, api.Get(t, "/providers/not-a-uuid"), 400, "VALIDATION", "provider_id must be a valid UUID")
		ws05Expect(t, api.Delete(t, "/providers/not-a-uuid"), 400, "VALIDATION", "provider_id must be a valid UUID")
		ws05Expect(t, api.Get(t, "/providers/"+ws05Missing), 404, "NOT_FOUND", "provider not found")
	})
	t.Run("empty update is a no-op", func(t *testing.T) {
		before := api.Get(t, "/providers/"+id).Map(t)
		ws05Expect(t, api.Put(t, "/providers/"+id, map[string]any{}), 204, "", "")
		after := api.Get(t, "/providers/"+id).Map(t)
		for _, k := range []string{"name", "protocol", "base_url", "status"} {
			if before[k] != after[k] {
				t.Errorf("%s changed: %v → %v", k, before[k], after[k])
			}
		}
	})
	t.Run("very long name round-trips", func(t *testing.T) {
		long := "ws05-" + strings.Repeat("ł", 2000)
		pid := ws05Provider(t, map[string]any{"name": long})
		if got := api.Get(t, "/providers/"+pid).Map(t)["name"]; got != long {
			t.Errorf("long name not preserved (len %d)", len(fmt.Sprint(got)))
		}
	})
}

// FINDING WS05-5: base_url is never validated as a URL, and an update can blank it
// (create requires it) — the provider then silently routes to the protocol default.
func TestWS05_ProviderBaseURLValidation(t *testing.T) {
	api := AdminAPI(t)
	r := api.Post(t, "/providers", map[string]any{"name": Uniq("ws05-badurl"), "protocol": "openai", "base_url": "not a url"})
	if r.Status == http.StatusCreated {
		var out struct{ ID string }
		r.JSON(t, &out)
		t.Cleanup(func() { api.Delete(t, "/providers/"+out.ID) })
	}
	if r.Status != 400 {
		t.Errorf("FINDING WS05-5: base_url %q accepted: %d %s", "not a url", r.Status, r.Body)
	}

	id := ws05Provider(t, map[string]any{"name": Uniq("ws05-blankurl")})
	r = api.Put(t, "/providers/"+id, map[string]any{"base_url": ""})
	if r.Status != 400 {
		got := api.Get(t, "/providers/"+id).Map(t)["base_url"]
		t.Errorf("FINDING WS05-5: update base_url=\"\" accepted (%d); stored base_url=%q", r.Status, got)
	}
}

// FINDING WS05-10: provider names are not unique.
func TestWS05_ProviderDuplicateName(t *testing.T) {
	name := Uniq("ws05-dupprov")
	ws05Provider(t, map[string]any{"name": name})
	r := AdminAPI(t).Post(t, "/providers", map[string]any{"name": name, "protocol": "openai", "base_url": "http://localhost:29100/ws05/openai/v1"})
	if r.Status == http.StatusCreated {
		var out struct{ ID string }
		r.JSON(t, &out)
		t.Cleanup(func() { AdminAPI(t).Delete(t, "/providers/"+out.ID) })
	}
	if r.Status != http.StatusConflict {
		t.Errorf("FINDING WS05-10: duplicate provider name accepted: %d %s", r.Status, r.Body)
	}
}

// Deleting a provider silently cascades its keys and mappings (no 409, no
// warning). This asserts the actual behaviour; recorded as FINDING WS05-4.
func TestWS05_ProviderDeleteCascades(t *testing.T) {
	api := AdminAPI(t)
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-casc")})
	key := ws05Key(t, p)
	m := ws05Model(t, Uniq("ws05-casc-m"))
	mp := ws05Mapping(t, map[string]any{"model_id": m, "provider_id": p, "external_id": "fake-ok"})

	ws05Expect(t, api.Delete(t, "/providers/"+p), 204, "", "")

	ws05Expect(t, api.Get(t, "/mappings/"+mp), 404, "NOT_FOUND", "")
	if r := api.Get(t, "/provider-keys/"+key); r.Status != 404 {
		t.Errorf("provider key survived provider delete: %d %s", r.Status, r.Body)
	}
	if items, total := ws05List(t, "/provider-keys?provider_id="+p); total != 0 {
		t.Errorf("keys still listed for deleted provider: %v", items)
	}
	// the model itself is untouched
	ws05Expect(t, api.Get(t, "/models/"+m), 200, "", "")
}

// ── models ──────────────────────────────────────────────────────────

func TestWS05_ModelCRUD(t *testing.T) {
	api := AdminAPI(t)
	name := Uniq("ws05-model") + "-日本"
	id := ws05Model(t, name)

	m := api.Get(t, "/models/"+id).Map(t)
	for k, want := range map[string]any{"name": name, "family": "ws05", "description": "ws05 test", "stability": "stable", "status": "active", "free": false} {
		if m[k] != want {
			t.Errorf("%s: want %v, got %v", k, want, m[k])
		}
	}
	items, total := ws05List(t, "/models?search="+name)
	if total != 1 || items[0]["id"] != id {
		t.Errorf("search: %d %v", total, items)
	}
	items, _ = ws05List(t, "/models?family=ws05&limit=100")
	found := false
	for _, it := range items {
		found = found || it["id"] == id
		if it["family"] != "ws05" {
			t.Errorf("family filter leaked %v", it["family"])
		}
	}
	if !found {
		t.Errorf("family filter missed model")
	}

	ws05Expect(t, api.Put(t, "/models/"+id, map[string]any{"stability": "beta", "status": "inactive", "free": true, "name": name + "-v2"}), 204, "", "")
	m = api.Get(t, "/models/"+id).Map(t)
	if m["stability"] != "beta" || m["status"] != "inactive" || m["free"] != true || m["name"] != name+"-v2" || m["family"] != "ws05" {
		t.Errorf("after update: %v", m)
	}
	items, _ = ws05List(t, "/models?status=inactive&search="+name)
	if len(items) != 1 {
		t.Errorf("status=inactive filter: %v", items)
	}

	ws05Expect(t, api.Delete(t, "/models/"+id), 204, "", "")
	ws05Expect(t, api.Get(t, "/models/"+id), 404, "NOT_FOUND", "model not found")
	ws05Expect(t, api.Delete(t, "/models/"+id), 404, "NOT_FOUND", "")
	ws05Expect(t, api.Put(t, "/models/"+id, map[string]any{"free": true}), 404, "NOT_FOUND", "")
}

func TestWS05_ModelValidation(t *testing.T) {
	api := AdminAPI(t)
	for _, c := range []struct {
		name string
		body any
		msg  string
	}{
		{"empty name", map[string]any{"name": "", "family": "ws05"}, "model name is required"},
		{"whitespace name", map[string]any{"name": "   ", "family": "ws05"}, "model name is required"},
		{"empty family", map[string]any{"name": "ws05-x", "family": ""}, "model family is required"},
		{"malformed", `[1,2]`, "invalid request body"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ws05Expect(t, api.Post(t, "/models", c.body), 400, "VALIDATION", c.msg)
		})
	}
	id := ws05Model(t, Uniq("ws05-mv"))
	for _, c := range []struct {
		name string
		body any
		msg  string
	}{
		{"empty name", map[string]any{"name": ""}, "model name cannot be empty"},
		{"bad stability", map[string]any{"stability": "alpha"}, "invalid model stability"},
		{"bad status", map[string]any{"status": "deleted"}, "invalid model status"},
	} {
		t.Run("update "+c.name, func(t *testing.T) {
			ws05Expect(t, api.Put(t, "/models/"+id, c.body), 400, "VALIDATION", c.msg)
		})
	}
	ws05Expect(t, api.Get(t, "/models/xyz"), 400, "VALIDATION", "model_id must be a valid UUID")
}

// FINDING WS05-2: model names are not unique, yet the gateway resolves models
// by name — duplicates make routing ambiguous and /v1/models lists the id twice.
func TestWS05_ModelDuplicateName(t *testing.T) {
	name := Uniq("ws05-dupmodel")
	ws05Model(t, name)
	r := AdminAPI(t).Post(t, "/models", map[string]any{"name": name, "family": "ws05"})
	if r.Status == http.StatusCreated {
		var out struct{ ID string }
		r.JSON(t, &out)
		t.Cleanup(func() { AdminAPI(t).Delete(t, "/models/"+out.ID) })
		if n := ws05Count(ws05GatewayModelIDs(t), name); n != 1 {
			t.Errorf("FINDING WS05-2: /v1/models lists %q %d times", name, n)
		}
	}
	if r.Status != http.StatusConflict {
		t.Errorf("FINDING WS05-2: duplicate model name accepted: %d %s", r.Status, r.Body)
	}
}

func TestWS05_ModelDeleteCascades(t *testing.T) {
	api := AdminAPI(t)
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-mdc")})
	a, b, c := ws05Model(t, Uniq("ws05-mdc-a")), ws05Model(t, Uniq("ws05-mdc-b")), ws05Model(t, Uniq("ws05-mdc-c"))
	mp := ws05Mapping(t, map[string]any{"model_id": b, "provider_id": p, "external_id": "fake-ok"})
	ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a, "fallback_model_id": b, "priority": 1}))
	ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": b, "fallback_model_id": c, "priority": 1}))

	ws05Expect(t, api.Delete(t, "/models/"+b), 204, "", "")
	ws05Expect(t, api.Get(t, "/mappings/"+mp), 404, "NOT_FOUND", "")
	if fb := ws05Fallbacks(t, a); len(fb) != 0 {
		t.Errorf("fallback pointing at deleted model survived: %v", fb)
	}
	if fb := ws05Fallbacks(t, b); len(fb) != 0 {
		t.Errorf("fallbacks of deleted model survived: %v", fb)
	}
	ws05Expect(t, api.Get(t, "/models/"+c), 200, "", "")
	ws05Expect(t, api.Get(t, "/providers/"+p), 200, "", "")
}

// ── mappings ────────────────────────────────────────────────────────

func TestWS05_MappingCRUD(t *testing.T) {
	api := AdminAPI(t)
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-map")})
	m := ws05Model(t, Uniq("ws05-map-m"))
	id := ws05Mapping(t, map[string]any{
		"model_id": m, "provider_id": p, "external_id": "fake-ok", "region": "eu-west",
		"input_price": 1.25, "output_price": 2.5, "cached_input_price": 0.1, "context_size": 128000, "max_output": 4096,
		"streaming": true, "tools": true, "vision": true,
	})
	r := api.Get(t, "/mappings/"+id).Map(t)
	for k, want := range map[string]any{
		"model_id": m, "provider_id": p, "external_id": "fake-ok", "region": "eu-west", "input_price": 1.25, "output_price": 2.5,
		"cached_input_price": 0.1, "context_size": float64(128000), "max_output": float64(4096), "streaming": true, "tools": true,
		"vision": true, "reasoning": false, "status": "active", "stability": "stable",
	} {
		if r[k] != want {
			t.Errorf("%s: want %v, got %v", k, want, r[k])
		}
	}
	if _, ok := r["request_price"]; ok {
		t.Errorf("unset price should be omitted, got request_price=%v", r["request_price"])
	}

	items, total := ws05List(t, "/mappings?model_id="+m)
	if total != 1 || items[0]["id"] != id {
		t.Errorf("filter by model: %d %v", total, items)
	}
	if _, total = ws05List(t, "/mappings?provider_id="+p); total != 1 {
		t.Errorf("filter by provider: %d", total)
	}
	ws05Expect(t, api.Get(t, "/mappings?model_id=bad"), 400, "VALIDATION", "model_id must be a valid UUID")
	ws05Expect(t, api.Get(t, "/mappings?provider_id=bad"), 400, "VALIDATION", "provider_id must be a valid UUID")

	ws05Expect(t, api.Put(t, "/mappings/"+id, map[string]any{"external_id": "fake-nousage", "output_price": 3.0, "tools": false, "status": "inactive"}), 204, "", "")
	r = api.Get(t, "/mappings/"+id).Map(t)
	if r["external_id"] != "fake-nousage" || r["output_price"] != 3.0 || r["tools"] != false || r["status"] != "inactive" || r["input_price"] != 1.25 {
		t.Errorf("after update: %v", r)
	}
	if _, total = ws05List(t, "/mappings?model_id="+m+"&status=inactive"); total != 1 {
		t.Errorf("status filter: %d", total)
	}

	for _, c := range []struct {
		body any
		msg  string
	}{
		{map[string]any{"external_id": " "}, "external_id cannot be empty"},
		{map[string]any{"status": "gone"}, "invalid mapping status"},
	} {
		ws05Expect(t, api.Put(t, "/mappings/"+id, c.body), 400, "VALIDATION", c.msg)
	}

	ws05Expect(t, api.Delete(t, "/mappings/"+id), 204, "", "")
	ws05Expect(t, api.Get(t, "/mappings/"+id), 404, "NOT_FOUND", "")
	ws05Expect(t, api.Delete(t, "/mappings/"+id), 404, "NOT_FOUND", "")
	ws05Expect(t, api.Put(t, "/mappings/"+id, map[string]any{"tools": true}), 404, "NOT_FOUND", "")
}

func TestWS05_MappingValidationAndConflicts(t *testing.T) {
	api := AdminAPI(t)
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-mapv")})
	m := ws05Model(t, Uniq("ws05-mapv-m"))
	for _, c := range []struct {
		name string
		body any
		msg  string
	}{
		{"missing model", map[string]any{"provider_id": p, "external_id": "x"}, "model_id is required"},
		{"missing provider", map[string]any{"model_id": m, "external_id": "x"}, "provider_id is required"},
		{"empty external_id", map[string]any{"model_id": m, "provider_id": p, "external_id": "  "}, "external_id is required"},
		{"non-uuid model", map[string]any{"model_id": "nope", "provider_id": p, "external_id": "x"}, "invalid request body"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ws05Expect(t, api.Post(t, "/mappings", c.body), 400, "VALIDATION", c.msg)
		})
	}

	ws05Mapping(t, map[string]any{"model_id": m, "provider_id": p, "external_id": "fake-ok", "region": "us"})
	t.Run("duplicate model+provider+region is 409", func(t *testing.T) {
		ws05Expect(t, api.Post(t, "/mappings", map[string]any{"model_id": m, "provider_id": p, "external_id": "other", "region": "us"}), 409, "CONFLICT", "mapping already exists")
	})
	t.Run("update to conflicting region is 409", func(t *testing.T) {
		id := ws05Mapping(t, map[string]any{"model_id": m, "provider_id": p, "external_id": "fake-ok", "region": "ap"})
		ws05Expect(t, api.Put(t, "/mappings/"+id, map[string]any{"region": "us"}), 409, "CONFLICT", "mapping already exists")
		if got := api.Get(t, "/mappings/"+id).Map(t)["region"]; got != "ap" {
			t.Errorf("region changed despite conflict: %v", got)
		}
	})
}

// FINDING WS05-7: (model, provider, region) uniqueness does not hold when region
// is omitted — NULLs are distinct in the UNIQUE constraint.
func TestWS05_MappingDuplicateWithoutRegion(t *testing.T) {
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-mapdup")})
	m := ws05Model(t, Uniq("ws05-mapdup-m"))
	ws05Mapping(t, map[string]any{"model_id": m, "provider_id": p, "external_id": "fake-ok"})
	r := AdminAPI(t).Post(t, "/mappings", map[string]any{"model_id": m, "provider_id": p, "external_id": "fake-ok"})
	if r.Status == http.StatusCreated {
		var out struct{ ID string }
		r.JSON(t, &out)
		t.Cleanup(func() { AdminAPI(t).Delete(t, "/mappings/"+out.ID) })
	}
	if r.Status != http.StatusConflict {
		t.Errorf("FINDING WS05-7: duplicate region-less mapping accepted: %d %s", r.Status, r.Body)
	}
}

// FINDING WS05-6: negative prices and limits are accepted.
func TestWS05_MappingNegativeValues(t *testing.T) {
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-mapneg")})
	m := ws05Model(t, Uniq("ws05-mapneg-m"))
	for i, body := range []map[string]any{
		{"input_price": -5.0},
		{"output_price": -0.01},
		{"context_size": -1},
		{"max_output": -100},
	} {
		body["model_id"], body["provider_id"], body["external_id"], body["region"] = m, p, "fake-ok", fmt.Sprintf("neg-%d", i)
		r := AdminAPI(t).Post(t, "/mappings", body)
		if r.Status == http.StatusCreated {
			var out struct{ ID string }
			r.JSON(t, &out)
			t.Cleanup(func() { AdminAPI(t).Delete(t, "/mappings/"+out.ID) })
		}
		if r.Status != 400 {
			t.Errorf("FINDING WS05-6: %v accepted: %d", body, r.Status)
		}
	}
	id := ws05Mapping(t, map[string]any{"model_id": m, "provider_id": p, "external_id": "fake-ok", "region": "neg-upd"})
	if r := AdminAPI(t).Put(t, "/mappings/"+id, map[string]any{"input_price": -1.0}); r.Status != 400 {
		t.Errorf("FINDING WS05-6: update input_price=-1 accepted: %d", r.Status)
	}
}

// FINDING WS05-3: references to non-existent models/providers return 500
// "database error" (FK violation) instead of 400/404.
func TestWS05_UnknownReferences(t *testing.T) {
	api := AdminAPI(t)
	p := ws05Provider(t, map[string]any{"name": Uniq("ws05-ref")})
	m := ws05Model(t, Uniq("ws05-ref-m"))
	for _, c := range []struct {
		name, path string
		body       map[string]any
	}{
		{"mapping unknown model", "/mappings", map[string]any{"model_id": ws05Missing, "provider_id": p, "external_id": "x"}},
		{"mapping unknown provider", "/mappings", map[string]any{"model_id": m, "provider_id": ws05Missing, "external_id": "x"}},
		{"fallback unknown fallback model", "/model-fallbacks", map[string]any{"model_id": m, "fallback_model_id": ws05Missing, "priority": 1}},
		{"fallback unknown model", "/model-fallbacks", map[string]any{"model_id": ws05Missing, "fallback_model_id": m, "priority": 1}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := api.Post(t, c.path, c.body)
			if r.Status != 404 && r.Status != 400 {
				t.Errorf("FINDING WS05-3: want 404/400, got %d %s", r.Status, r.Body)
			}
		})
	}
}

// ── fallbacks ───────────────────────────────────────────────────────

func TestWS05_FallbackRules(t *testing.T) {
	api := AdminAPI(t)
	a, b, c := ws05Model(t, Uniq("ws05-fb-a")), ws05Model(t, Uniq("ws05-fb-b")), ws05Model(t, Uniq("ws05-fb-c"))

	t.Run("validation", func(t *testing.T) {
		ws05Expect(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a, "fallback_model_id": a, "priority": 1}), 400, "VALIDATION", "model cannot be its own fallback")
		ws05Expect(t, api.Post(t, "/model-fallbacks", map[string]any{"fallback_model_id": a}), 400, "VALIDATION", "model_id is required")
		ws05Expect(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a}), 400, "VALIDATION", "fallback_model_id is required")
		ws05Expect(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": "x", "fallback_model_id": b}), 400, "VALIDATION", "invalid request body")
		ws05Expect(t, api.Get(t, "/model-fallbacks/by-model/bad"), 400, "VALIDATION", "model_id must be a valid UUID")
		ws05Expect(t, api.Delete(t, "/model-fallbacks/"+ws05Missing), 404, "NOT_FOUND", "fallback not found")
	})

	ab := ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a, "fallback_model_id": b, "priority": 2}))
	t.Run("duplicate pair is 409 regardless of priority", func(t *testing.T) {
		ws05Expect(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a, "fallback_model_id": b, "priority": 9}), 409, "CONFLICT", "fallback already exists")
	})
	t.Run("duplicate priority accepted, list ordered by priority", func(t *testing.T) {
		ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a, "fallback_model_id": c, "priority": 2}))
		fb := ws05Fallbacks(t, a)
		if len(fb) != 2 {
			t.Fatalf("want 2 fallbacks, got %v", fb)
		}
		for _, f := range fb {
			if f["priority"] != float64(2) || f["enabled"] != true || f["model_id"] != a {
				t.Errorf("unexpected fallback %v", f)
			}
		}
	})
	t.Run("negative priority accepted and sorts first", func(t *testing.T) {
		d := ws05Model(t, Uniq("ws05-fb-d"))
		ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": a, "fallback_model_id": d, "priority": -1}))
		if fb := ws05Fallbacks(t, a); len(fb) != 3 || fb[0]["fallback_model_id"] != d {
			t.Errorf("want d first by priority, got %v", fb)
		}
	})
	t.Run("cycle A→B→A is accepted (one-level resolution)", func(t *testing.T) {
		ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": b, "fallback_model_id": a, "priority": 1}))
	})
	t.Run("unknown model lists empty", func(t *testing.T) {
		if fb := ws05Fallbacks(t, ws05Missing); len(fb) != 0 {
			t.Errorf("want [], got %v", fb)
		}
	})
	t.Run("delete", func(t *testing.T) {
		ws05Expect(t, api.Delete(t, "/model-fallbacks/"+ab), 204, "", "")
		ws05Expect(t, api.Delete(t, "/model-fallbacks/"+ab), 404, "NOT_FOUND", "fallback not found")
		for _, f := range ws05Fallbacks(t, a) {
			if f["id"] == ab {
				t.Errorf("deleted fallback still listed")
			}
		}
	})
}

// A fallback cycle must not loop in the gateway: A(400)→B(500)→A resolves one level only.
func TestWS05_FallbackCycleTerminates(t *testing.T) {
	prefix := Uniq("ws05cyc")
	api := AdminAPI(t)
	p := ws05Routable(t, prefix)
	a := Uniq("ws05-cyc-a")
	aID, bID := ws05Model(t, a), ws05Model(t, Uniq("ws05-cyc-b"))
	ws05Mapping(t, map[string]any{"model_id": aID, "provider_id": p, "external_id": "fake-500", "output_price": 1.0})
	ws05Mapping(t, map[string]any{"model_id": bID, "provider_id": p, "external_id": "fake-500", "output_price": 1.0})
	ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": aID, "fallback_model_id": bID, "priority": 1}))
	ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": bID, "fallback_model_id": aID, "priority": 1}))

	f := FX(t)
	gw := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret)
	r := gw.Post(t, "/chat/completions", Chat(a, Uniq("ws05 cycle"), false))
	if r.Status != http.StatusBadGateway {
		t.Fatalf("want 502 after exhausting routes, got %d %s", r.Status, r.Body)
	}
	if got := ws05UpstreamModels(t, prefix); len(got) != 2 {
		t.Errorf("want exactly 2 upstream attempts (A then B), got %v", got)
	}
}

// FINDING WS05-1: under the default "cheapest" strategy, routes of fallback models
// are sorted together with the primary's routes, so a cheaper fallback model is
// served instead of a healthy primary.
func TestWS05_FallbackNotPreferredOverPrimary(t *testing.T) {
	prefix := Uniq("ws05ord")
	api := AdminAPI(t)
	p := ws05Routable(t, prefix)
	primary := Uniq("ws05-ord-primary")
	pID, fID := ws05Model(t, primary), ws05Model(t, Uniq("ws05-ord-fallback"))
	ws05Mapping(t, map[string]any{"model_id": pID, "provider_id": p, "external_id": "fake-ok", "output_price": 10.0})
	ws05Mapping(t, map[string]any{"model_id": fID, "provider_id": p, "external_id": "fake-nousage", "output_price": 1.0})
	ws05ID(t, api.Post(t, "/model-fallbacks", map[string]any{"model_id": pID, "fallback_model_id": fID, "priority": 1}))

	f := FX(t)
	r := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret).Post(t, "/chat/completions", Chat(primary, Uniq("ws05 order"), false))
	if r.Status != 200 {
		t.Fatalf("chat: %d %s", r.Status, r.Body)
	}
	got := ws05UpstreamModels(t, prefix)
	if len(got) == 0 || got[0] != "fake-ok" {
		t.Errorf("FINDING WS05-1: healthy primary not tried first; upstream calls %v", got)
	}
}

// ── permissions ─────────────────────────────────────────────────────

func TestWS05_Permissions(t *testing.T) {
	viewer := AsUser(t, "viewer")
	ws05Expect(t, viewer.Get(t, "/models?limit=1"), 200, "", "")
	ws05Expect(t, viewer.Get(t, "/mappings?limit=1"), 200, "", "")
	ws05Expect(t, viewer.Get(t, "/model-fallbacks/by-model/"+FX(t).Models["e2e-fallback"]), 200, "", "")
	for _, c := range []struct{ method, path string }{
		{"POST", "/providers"}, {"PUT", "/providers/" + FX(t).Providers["openai"]}, {"DELETE", "/providers/" + ws05Missing},
		{"POST", "/models"}, {"DELETE", "/models/" + ws05Missing},
		{"POST", "/mappings"}, {"DELETE", "/mappings/" + ws05Missing},
		{"POST", "/model-fallbacks"}, {"DELETE", "/model-fallbacks/" + ws05Missing},
	} {
		r := viewer.Do(t, c.method, c.path, map[string]any{"name": "ws05-forbidden"})
		if r.Status != http.StatusForbidden {
			t.Errorf("viewer %s %s: want 403, got %d %s", c.method, c.path, r.Status, r.Body)
		}
	}

	po := AsUser(t, "providers_only")
	id := ws05ID(t, po.Post(t, "/models", map[string]any{"name": Uniq("ws05-po"), "family": "ws05"}))
	t.Cleanup(func() { AdminAPI(t).Delete(t, "/models/"+id) })
	ws05Expect(t, po.Delete(t, "/models/"+id), 204, "", "")

	gw := Bearer(FX(t).URLs.API, FX(t).Personas["gw_key"].Secret)
	ws05Expect(t, gw.Get(t, "/models"), 403, "", "")
	ws05Expect(t, Anon(FX(t).URLs.API).Get(t, "/models"), 401, "", "")
}

// ── gateway: /v1/models + response cache vs catalog changes ─────────

// ws05GatewayKey creates a private gateway:invoke service account so that
// cache invalidation can be scoped to our own subject (?subject=<id>).
func ws05GatewayKey(t *testing.T) (id, secret string) {
	t.Helper()
	api := AdminAPI(t)
	var r Resp
	deadline := time.Now().Add(90 * time.Second)
	for {
		r = api.Post(t, "/service-accounts", map[string]any{"name": Uniq("ws05-gw"), "permissions": []string{"freerouter:gateway:invoke"}, "expires_in": "1h"})
		// IAMKit throttles /api/v1 per IP; shared by all workers (see WS04 findings).
		if r.Status != http.StatusBadGateway || time.Now().After(deadline) {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if r.Status != http.StatusCreated {
		t.Fatalf("create service account: %d %s", r.Status, r.Body)
	}
	var c struct{ ID, Secret string }
	r.JSON(t, &c)
	t.Cleanup(func() { api.Delete(t, "/service-accounts/"+c.ID) })
	return c.ID, c.Secret
}

func TestWS05_GatewayCatalogAndCache(t *testing.T) {
	prefix := Uniq("ws05cache")
	f := FX(t)
	api := AdminAPI(t)
	p := ws05Routable(t, prefix)
	name := Uniq("ws05-cache-m")
	mID := ws05Model(t, name)
	ws05Mapping(t, map[string]any{"model_id": mID, "provider_id": p, "external_id": "fake-ok", "output_price": 1.0})

	t.Run("/v1/models reflects create, deactivate, delete immediately", func(t *testing.T) {
		if n := ws05Count(ws05GatewayModelIDs(t), name); n != 1 {
			t.Fatalf("new model listed %d times", n)
		}
		ws05Expect(t, api.Put(t, "/models/"+mID, map[string]any{"status": "inactive"}), 204, "", "")
		if n := ws05Count(ws05GatewayModelIDs(t), name); n != 0 {
			t.Errorf("inactive model still listed")
		}
		ws05Expect(t, api.Put(t, "/models/"+mID, map[string]any{"status": "active"}), 204, "", "")
		if n := ws05Count(ws05GatewayModelIDs(t), name); n != 1 {
			t.Errorf("reactivated model not listed")
		}
	})

	subject, secret := ws05GatewayKey(t)
	gw := Bearer(f.URLs.Gateway, secret)
	body := Chat(name, Uniq("ws05 cache probe"), false)

	r := gw.Post(t, "/chat/completions", body)
	if r.Status != 200 || r.Header.Get("X-Cache") != "MISS" {
		t.Fatalf("first chat: %d X-Cache=%q %s", r.Status, r.Header.Get("X-Cache"), r.Body)
	}
	r = gw.Post(t, "/chat/completions", body)
	if r.Status != 200 || r.Header.Get("X-Cache") != "HIT" {
		t.Fatalf("second chat: %d X-Cache=%q", r.Status, r.Header.Get("X-Cache"))
	}

	// Remove the model from the catalog, then repeat the identical request
	// (well within CACHE_TTL_SECONDS).
	ws05Expect(t, api.Delete(t, "/models/"+mID), 204, "", "")
	if n := ws05Count(ws05GatewayModelIDs(t), name); n != 0 {
		t.Errorf("deleted model still in /v1/models")
	}
	// Actual behaviour (FINDING WS05-9, low): catalog writes do not invalidate
	// the response cache, so the deleted model keeps answering until TTL or an
	// explicit DELETE /gateway/cache.
	t.Run("deleted model still served from cache until invalidated", func(t *testing.T) {
		r := gw.Post(t, "/chat/completions", body)
		if r.Status != 200 || r.Header.Get("X-Cache") != "HIT" {
			t.Errorf("want stale 200 HIT, got %d X-Cache=%q %s", r.Status, r.Header.Get("X-Cache"), r.Body)
		}
	})

	t.Run("subject-scoped DELETE /gateway/cache clears it", func(t *testing.T) {
		ws05Expect(t, AsUser(t, "viewer").Delete(t, "/gateway/cache?subject="+subject), 403, "", "")
		inv := api.Delete(t, "/gateway/cache?subject="+subject)
		ws05Expect(t, inv, 200, "", "")
		if n, _ := inv.Map(t)["invalidated"].(float64); n < 1 {
			t.Errorf("want ≥1 invalidated, got %s", inv.Body)
		}
		r := gw.Post(t, "/chat/completions", body)
		ws05Expect(t, r, 404, "NOT_FOUND", "model not found")
		if r.Header.Get("X-Cache") != "MISS" {
			t.Errorf("want X-Cache MISS, got %q", r.Header.Get("X-Cache"))
		}
	})
}
