//go:build e2e

package api

// WS-08 helpers: one own gateway service account (subject) with its own
// per-subject rate limit, own fakellm-backed catalog under /ws08/<seg>/...,
// guardrail config snapshot/restore, sink filtering and Redis/Postgres access.
// Everything created here is removed in t.Cleanup or TestWS08_ZZZ_Cleanup.

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const (
	ws08BaseRPM = 5000
	ws08BaseMC  = 100
)

// ws08GW is the WS-08 gateway caller (own IAMKit subject + own rate limit row).
type ws08GW struct {
	*Client
	Subject string
	Secret  string
	RLID    string // id of the subject's rate-limit config
}

var (
	ws08Pool   *ws08GW
	ws08PoolMu sync.Mutex
)

// ws08Gateway returns the shared WS-08 caller, creating it on first use.
// IAMKit throttles service-account creation and machine-token exchange, so
// one account is created per run and revoked by TestWS08_ZZZ_Cleanup.
func ws08Gateway(t *testing.T) ws08GW {
	t.Helper()
	ws08PoolMu.Lock()
	defer ws08PoolMu.Unlock()
	if ws08Pool != nil {
		return *ws08Pool
	}
	f := FX(t)
	admin := AdminAPI(t)
	var r Resp
	for i := 0; i < 12; i++ {
		r = admin.Post(t, "/service-accounts", map[string]any{
			"name": Uniq("ws08-gw"), "permissions": []string{"freerouter:gateway:invoke"},
		})
		if r.Status == http.StatusCreated {
			break
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if r.Status != http.StatusCreated {
		t.Fatalf("create service account (IAMKit throttled?): %d %s", r.Status, r.Body)
	}
	var sa struct{ ID, Secret string }
	r.JSON(t, &sa)
	rl := admin.Post(t, "/rate-limits", map[string]any{
		"name": Uniq("ws08-rl"), "subject_id": sa.ID, "rpm": ws08BaseRPM, "max_concurrent": ws08BaseMC,
	})
	if rl.Status != http.StatusCreated {
		t.Fatalf("create rate limit: %d %s", rl.Status, rl.Body)
	}
	gw := &ws08GW{Client: Bearer(f.URLs.Gateway, sa.Secret), Subject: sa.ID, Secret: sa.Secret, RLID: rl.Map(t)["id"].(string)}
	ws08Pool = gw // register before warm-up so cleanup still revokes it
	for i := 0; i < 20; i++ {
		if g := gw.Get(t, "/models"); g.Status == 200 {
			return *gw
		}
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
	t.Fatalf("new service account never authenticated (IAMKit machine-token throttled)")
	return *gw
}

// ws08CleanupPool revokes the shared service account and its rate limit.
func ws08CleanupPool(t *testing.T) {
	ws08PoolMu.Lock()
	defer ws08PoolMu.Unlock()
	if ws08Pool == nil {
		return
	}
	admin := AdminAPI(t)
	admin.Delete(t, "/rate-limits/"+ws08Pool.RLID)
	ok := false
	for j := 0; j < 8 && !ok; j++ {
		d := admin.Delete(t, "/service-accounts/"+ws08Pool.Subject)
		ok = d.Status == http.StatusNoContent || d.Status == http.StatusNotFound
		if !ok {
			time.Sleep(time.Duration(j+1) * time.Second)
		}
	}
	if !ok {
		t.Errorf("could not revoke ws08 service account %s", ws08Pool.Subject)
	}
	ws08ResetRedis(t, ws08Pool.Subject)
	ws08Pool = nil
}

// ws08SetLimit patches the subject's limit and restores the generous default.
func ws08SetLimit(t *testing.T, gw ws08GW, rpm, mc int) {
	t.Helper()
	admin := AdminAPI(t)
	if r := admin.Patch(t, "/rate-limits/"+gw.RLID, map[string]any{"rpm": rpm, "max_concurrent": mc}); r.Status != 200 {
		t.Fatalf("patch rate limit: %d %s", r.Status, r.Body)
	}
	ws08ResetRedis(t, gw.Subject)
	t.Cleanup(func() {
		admin.Patch(t, "/rate-limits/"+gw.RLID, map[string]any{"rpm": ws08BaseRPM, "max_concurrent": ws08BaseMC})
		ws08ResetRedis(t, gw.Subject)
	})
}

// ── Redis / Postgres ─────────────────────────────────────────────────

var (
	ws08RedisOnce sync.Once
	ws08RDB       *redis.Client
	ws08DBOnce    sync.Once
	ws08DB        *sql.DB
	ws08DBErr     error
)

func ws08Redis(t *testing.T) *redis.Client {
	t.Helper()
	ws08RedisOnce.Do(func() { ws08RDB = redis.NewClient(&redis.Options{Addr: FX(t).URLs.Redis}) })
	if err := ws08RDB.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis: %v", err)
	}
	return ws08RDB
}

// ws08ResetRedis clears the limiter keys of the WS-08 subject only.
func ws08ResetRedis(t *testing.T, subject string) {
	t.Helper()
	ws08Redis(t).Del(context.Background(), "ratelimit:rpm:"+subject, "ratelimit:concurrent:"+subject)
}

func ws08Postgres(t *testing.T) *sql.DB {
	t.Helper()
	ws08DBOnce.Do(func() {
		ws08DB, ws08DBErr = sql.Open("postgres", FX(t).URLs.DB)
		if ws08DBErr == nil {
			ws08DBErr = ws08DB.Ping()
		}
	})
	if ws08DBErr != nil {
		t.Fatalf("postgres: %v", ws08DBErr)
	}
	return ws08DB
}

// ── Catalog ──────────────────────────────────────────────────────────

// ws08Model creates provider (fakellm /ws08/<seg>/openai/v1 or base) + key +
// model + mapping to external. Returns the model name.
func ws08Model(t *testing.T, seg, external string, base ...string) string {
	t.Helper()
	f := FX(t)
	admin := AdminAPI(t)
	url := f.URLs.FakeLLM + "/ws08/" + seg + "/openai/v1"
	if len(base) > 0 {
		url = base[0]
	}
	r := admin.Post(t, "/providers", map[string]any{
		"name": Uniq("ws08-" + seg), "protocol": "openai", "base_url": url, "description": "ws08", "streaming": true,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider: %d %s", r.Status, r.Body)
	}
	pid := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/providers/"+pid) })
	if k := admin.Post(t, "/provider-keys", map[string]any{
		"provider_id": pid, "name": "ws08-key-" + seg, "key_type": "api_key", "token": "ws08-tok-" + seg,
	}); k.Status != http.StatusCreated {
		t.Fatalf("create provider key: %d %s", k.Status, k.Body)
	}
	name := Uniq("ws08-" + seg)
	m := admin.Post(t, "/models", map[string]any{"name": name, "family": "ws08", "description": "ws08"})
	if m.Status != http.StatusCreated {
		t.Fatalf("create model: %d %s", m.Status, m.Body)
	}
	mid := m.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/models/"+mid) })
	mp := admin.Post(t, "/mappings", map[string]any{
		"model_id": mid, "provider_id": pid, "external_id": external,
		"input_price": 1.0, "output_price": 2.0, "streaming": true, "tools": true, "json_output": true,
	})
	if mp.Status != http.StatusCreated {
		t.Fatalf("create mapping: %d %s", mp.Status, mp.Body)
	}
	return name
}

// ws08Rec is one request recorded by fakellm.
type ws08Rec struct {
	Path string          `json:"path"`
	Body json.RawMessage `json:"body"`
}

// ws08Upstream returns fakellm requests under /ws08/<seg>/.
func ws08Upstream(t *testing.T, seg string) []ws08Rec {
	t.Helper()
	var all []ws08Rec
	Anon(FX(t).URLs.FakeLLM).Get(t, "/_requests").JSON(t, &all)
	var out []ws08Rec
	for _, r := range all {
		if strings.HasPrefix(r.Path, "/ws08/"+seg+"/") {
			out = append(out, r)
		}
	}
	return out
}

// ws08Slow is an in-process upstream that holds every request until
// release is closed (or hold elapses), counting requests in flight.
type ws08Slow struct {
	URL     string
	mu      sync.Mutex
	hits    int
	release chan struct{}
}

func ws08NewSlow(t *testing.T, hold time.Duration) *ws08Slow {
	t.Helper()
	s := &ws08Slow{release: make(chan struct{})}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		s.mu.Lock()
		s.hits++
		s.mu.Unlock()
		select {
		case <-s.release:
		case <-time.After(hold):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-ws08", "object": "chat.completion", "created": time.Now().Unix(), "model": "slow",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	t.Cleanup(func() {
		s.Release()
		srv.CloseClientConnections()
		srv.Close()
	})
	s.URL = srv.URL
	return s
}

func (s *ws08Slow) Hits() int { s.mu.Lock(); defer s.mu.Unlock(); return s.hits }

var ws08ReleaseMu sync.Mutex

func (s *ws08Slow) Release() {
	ws08ReleaseMu.Lock()
	defer ws08ReleaseMu.Unlock()
	select {
	case <-s.release:
	default:
		close(s.release)
	}
}

// ── Guardrails ───────────────────────────────────────────────────────

var ws08AllOff = map[string]any{
	"prompt_injection": map[string]any{"enabled": false, "action": "block"},
	"jailbreak":        map[string]any{"enabled": false, "action": "block"},
	"pii_detection":    map[string]any{"enabled": false, "action": "redact"},
	"secrets":          map[string]any{"enabled": false, "action": "block"},
	"document_leakage": map[string]any{"enabled": false, "action": "warn"},
}

// ws08Rules returns ws08AllOff with the given detectors switched on.
func ws08Rules(on map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range ws08AllOff {
		out[k] = v
	}
	for k, action := range on {
		out[k] = map[string]any{"enabled": true, "action": action}
	}
	return out
}

// ws08GuardrailSnapshot records the global guardrail config and registers
// a cleanup that restores it exactly: PUT the old values back, or — when
// no config row existed — delete the row(s) the test created.
func ws08GuardrailSnapshot(t *testing.T) {
	t.Helper()
	admin := AdminAPI(t)
	r := admin.Get(t, "/guardrails/config")
	switch r.Status {
	case http.StatusOK:
		var old struct {
			Enabled     bool            `json:"enabled"`
			SystemRules json.RawMessage `json:"system_rules"`
		}
		r.JSON(t, &old)
		t.Cleanup(func() {
			if p := admin.Put(t, "/guardrails/config", map[string]any{"enabled": old.Enabled, "system_rules": old.SystemRules}); p.Status != 200 {
				t.Errorf("restore guardrail config: %d %s", p.Status, p.Body)
			}
		})
	case http.StatusNotFound:
		var before []string
		rows, err := ws08Postgres(t).Query(`SELECT id::text FROM guardrail_configs`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			before = append(before, id)
		}
		rows.Close()
		t.Cleanup(func() {
			// Product has no DELETE for the singleton; remove what this test created.
			if _, err := ws08Postgres(t).Exec(`DELETE FROM guardrail_configs WHERE NOT (id::text = ANY($1))`, "{"+strings.Join(before, ",")+"}"); err != nil {
				t.Errorf("restore (delete) guardrail config: %v", err)
			}
		})
	default:
		t.Fatalf("GET guardrail config: %d %s", r.Status, r.Body)
	}
}

// ws08Guardrails enables the global config with the given detectors on.
func ws08Guardrails(t *testing.T, enabled bool, on map[string]string) {
	t.Helper()
	r := AdminAPI(t).Put(t, "/guardrails/config", map[string]any{"enabled": enabled, "system_rules": ws08Rules(on)})
	if r.Status != 200 {
		t.Fatalf("put guardrail config: %d %s", r.Status, r.Body)
	}
}

// ws08Rule creates a custom rule and deletes it on cleanup.
func ws08Rule(t *testing.T, body map[string]any) string {
	t.Helper()
	admin := AdminAPI(t)
	r := admin.Post(t, "/guardrails/rules", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create rule: %d %s", r.Status, r.Body)
	}
	id := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/guardrails/rules/"+id) })
	return id
}

// ws08Violations returns recent violations (newest first).
func ws08Violations(t *testing.T) []map[string]any {
	t.Helper()
	var page struct {
		Items []map[string]any `json:"items"`
	}
	AdminAPI(t).Get(t, "/guardrails/violations?limit=100").JSON(t, &page)
	return page.Items
}

// ── Webhooks ─────────────────────────────────────────────────────────

type ws08Hook struct {
	ID, Secret, Path string
}

// ws08Webhook subscribes sink path (e.g. "/hook/ws08-x-123") to events.
func ws08Webhook(t *testing.T, path string, events ...string) ws08Hook {
	t.Helper()
	admin := AdminAPI(t)
	r := admin.Post(t, "/webhooks", map[string]any{"url": FX(t).URLs.WebhookSink + path, "events": events})
	if r.Status != http.StatusCreated {
		t.Fatalf("create webhook: %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	h := ws08Hook{ID: m["id"].(string), Path: path}
	h.Secret, _ = m["secret"].(string)
	t.Cleanup(func() { admin.Delete(t, "/webhooks/"+h.ID) })
	return h
}

// ws08Delivery is one sink recording.
type ws08Delivery struct {
	At      time.Time         `json:"at"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	Status  int               `json:"status"`
}

func ws08Sink(t *testing.T, path string) []ws08Delivery {
	t.Helper()
	var all []ws08Delivery
	Anon(FX(t).URLs.WebhookSink).Get(t, "/_deliveries").JSON(t, &all)
	var out []ws08Delivery
	for _, d := range all {
		if d.Path == path {
			out = append(out, d)
		}
	}
	return out
}
