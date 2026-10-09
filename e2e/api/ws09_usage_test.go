//go:build e2e

package api

// WS-09 — usage logs, summary and retention.
//
// Other workstreams generate usage concurrently, so nothing here asserts on
// global totals: every test owns a provider (fakellm under /ws09/<seg>/), its
// own models/mappings with distinctive prices and its own gateway service
// account, and filters usage by its unique model names.
//
// Cost arithmetic (internal/gateway/adapters/gatewayhttp/handler.go logUsage):
//   input_cost  = prompt_tokens     * input_price  / 1_000_000
//   output_cost = completion_tokens * output_price / 1_000_000
// fakellm fake-ok answers prompt 5 / completion 4 / total 9.

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

const (
	ws09InPrice  = 3.0 // USD per 1M prompt tokens
	ws09OutPrice = 7.0 // USD per 1M completion tokens
)

// ws09Log mirrors usage.UsageLog JSON.
type ws09Log struct {
	ID               string    `json:"id"`
	KeyID            string    `json:"key_id"`
	RequestedModel   string    `json:"requested_model"`
	UsedModel        string    `json:"used_model"`
	ProviderID       string    `json:"provider_id"`
	MappingID        string    `json:"mapping_id"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CachedTokens     int       `json:"cached_tokens"`
	InputCost        float64   `json:"input_cost"`
	OutputCost       float64   `json:"output_cost"`
	TotalCost        float64   `json:"total_cost"`
	DurationMs       int       `json:"duration_ms"`
	Streamed         bool      `json:"streamed"`
	StatusCode       int       `json:"status_code"`
	FinishReason     string    `json:"finish_reason"`
	HasError         bool      `json:"has_error"`
	ErrorMessage     string    `json:"error_message"`
	IsFallback       bool      `json:"is_fallback"`
	CreatedAt        time.Time `json:"created_at"`
}

type ws09Page struct {
	Items []ws09Log `json:"items"`
	Page  struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"page"`
}

type ws09ModelSummary struct {
	Model            string  `json:"model"`
	TotalRequests    int     `json:"total_requests"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalCost        float64 `json:"total_cost"`
}

type ws09Summary struct {
	Summary struct {
		TotalRequests    int     `json:"total_requests"`
		TotalTokens      int     `json:"total_tokens"`
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalCost        float64 `json:"total_cost"`
		ErrorCount       int     `json:"error_count"`
	} `json:"summary"`
	ByModel     []ws09ModelSummary `json:"by_model"`
	PeriodStart time.Time          `json:"period_start"`
	PeriodEnd   time.Time          `json:"period_end"`
}

func (s ws09Summary) model(name string) (ws09ModelSummary, bool) {
	for _, m := range s.ByModel {
		if m.Model == name {
			return m, true
		}
	}
	return ws09ModelSummary{}, false
}

type ws09Model struct{ ID, Name, MappingID, External string }

// ws09Env is one isolated slice of the catalog + a dedicated gateway caller.
type ws09Env struct {
	GW       *Client
	SAID     string
	SASecret string
	ProvID   string
	KeyID    string
	Token    string // provider upstream token (sk-ws09-…), must never be logged
	Seg      string
	Models   map[string]ws09Model // ok, nousage, e500, e400
}

func ws09Approx(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func ws09Cost(prompt, completion int) (in, out float64) {
	return float64(prompt) * ws09InPrice / 1_000_000, float64(completion) * ws09OutPrice / 1_000_000
}

// ws09Retry repeats IAMKit-backed admin calls that occasionally 502 under the shared login/introspection load.
func ws09Retry(t *testing.T, fn func() Resp, ok ...int) Resp {
	t.Helper()
	var r Resp
	for i := 0; i < 6; i++ {
		r = fn()
		for _, s := range ok {
			if r.Status == s {
				return r
			}
		}
		if r.Status != http.StatusBadGateway && r.Status != http.StatusTooManyRequests {
			return r
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return r
}

// ws09NewEnv creates provider + key + models (ok/nousage/500/400) + gateway service account.
func ws09NewEnv(t *testing.T, seg string) ws09Env {
	t.Helper()
	f := FX(t)
	admin := AdminAPI(t)
	seg = Uniq("ws09" + seg)
	e := ws09Env{Seg: seg, Token: "sk-ws09-" + strings.TrimPrefix(seg, "ws09"), Models: map[string]ws09Model{}}

	r := admin.Post(t, "/providers", map[string]any{
		"name": seg, "protocol": "openai", "base_url": f.URLs.FakeLLM + "/ws09/" + seg + "/openai/v1",
		"description": "ws09", "streaming": true,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider: %d %s", r.Status, r.Body)
	}
	e.ProvID = r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/providers/"+e.ProvID) })

	r = admin.Post(t, "/provider-keys", map[string]any{
		"provider_id": e.ProvID, "name": seg + "-key", "key_type": "api_key", "token": e.Token,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider key: %d %s", r.Status, r.Body)
	}
	e.KeyID = r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/provider-keys/"+e.KeyID) })

	for kind, ext := range map[string]string{"ok": "fake-ok", "nousage": "fake-nousage", "e500": "fake-500", "e400": "fake-400"} {
		name := seg + "-" + kind
		r := admin.Post(t, "/models", map[string]any{"name": name, "family": "ws09", "description": "ws09"})
		if r.Status != http.StatusCreated {
			t.Fatalf("create model: %d %s", r.Status, r.Body)
		}
		m := ws09Model{ID: r.Map(t)["id"].(string), Name: name, External: ext}
		t.Cleanup(func() { admin.Delete(t, "/models/"+m.ID) })
		r = admin.Post(t, "/mappings", map[string]any{
			"model_id": m.ID, "provider_id": e.ProvID, "external_id": ext,
			"input_price": ws09InPrice, "output_price": ws09OutPrice, "streaming": true,
		})
		if r.Status != http.StatusCreated {
			t.Fatalf("create mapping: %d %s", r.Status, r.Body)
		}
		m.MappingID = r.Map(t)["id"].(string)
		e.Models[kind] = m
	}

	r = ws09Retry(t, func() Resp {
		return admin.Post(t, "/service-accounts", map[string]any{
			"name": Uniq("ws09-gw"), "permissions": []string{"freerouter:gateway:invoke"},
		})
	}, http.StatusCreated, http.StatusOK)
	if r.Status != http.StatusCreated && r.Status != http.StatusOK {
		t.Fatalf("create service account: %d %s", r.Status, r.Body)
	}
	var sa struct{ ID, Secret string }
	r.JSON(t, &sa)
	e.SAID, e.SASecret = sa.ID, sa.Secret
	t.Cleanup(func() {
		ws09Retry(t, func() Resp { return admin.Delete(t, "/service-accounts/"+sa.ID) }, http.StatusNoContent, http.StatusNotFound)
	})
	rl := admin.Post(t, "/rate-limits", map[string]any{
		"name": Uniq("ws09-rl"), "subject_id": sa.ID, "rpm": 1000, "max_concurrent": 50,
	})
	if rl.Status != http.StatusCreated {
		t.Fatalf("create rate limit: %d %s", rl.Status, rl.Body)
	}
	rlID := rl.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/rate-limits/"+rlID) })

	e.GW = Bearer(f.URLs.Gateway, sa.Secret)
	return e
}

// chat sends a non-streaming completion with a unique prompt (bypasses the response cache).
func (e ws09Env) chat(t *testing.T, kind string) Resp {
	t.Helper()
	body := Chat(e.Models[kind].Name, "ws09 "+Uniq("p"), false)
	var r Resp
	for i := 0; i < 5; i++ { // the very first introspection of a new service account can 502
		r = e.GW.Post(t, "/chat/completions", body)
		// retry only auth hiccups (401, or 502 not caused by the upstream)
		authHiccup := r.Status == http.StatusUnauthorized ||
			r.Status == http.StatusBadGateway && !strings.Contains(string(r.Body), "upstream")
		if !authHiccup {
			return r
		}
		time.Sleep(time.Second)
	}
	return r
}

// ws09Usage lists usage with the given raw query string.
func ws09Usage(t *testing.T, q string) ws09Page {
	t.Helper()
	r := AdminAPI(t).Get(t, "/usage?"+q)
	if r.Status != 200 {
		t.Fatalf("GET /usage?%s: %d %s", q, r.Status, r.Body)
	}
	var p ws09Page
	r.JSON(t, &p)
	return p
}

// ws09WaitLogs waits until n usage logs exist for model and returns them (newest first).
func ws09WaitLogs(t *testing.T, model string, n int) []ws09Log {
	t.Helper()
	var p ws09Page
	Eventually(t, 15*time.Second, func() bool {
		p = ws09Usage(t, "limit=100&model="+url.QueryEscape(model))
		return p.Page.Total >= n
	}, fmt.Sprintf("%d usage logs for %s", n, model))
	if p.Page.Total != n {
		t.Fatalf("usage logs for %s: want exactly %d, got %d", model, n, p.Page.Total)
	}
	return p.Items
}

func ws09Summ(t *testing.T, q string) ws09Summary {
	t.Helper()
	r := AdminAPI(t).Get(t, "/usage/summary?"+q)
	if r.Status != 200 {
		t.Fatalf("GET /usage/summary?%s: %d %s", q, r.Status, r.Body)
	}
	var s ws09Summary
	r.JSON(t, &s)
	return s
}

func ws09WantErr(t *testing.T, r Resp, status int, msg, what string) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("%s: want %d, got %d %s", what, status, r.Status, r.Body)
	}
	if msg != "" && !strings.Contains(string(r.Body), msg) {
		t.Fatalf("%s: body %s does not contain %q", what, r.Body, msg)
	}
}

// ─────────────────────────────────────────────────────────────────────

// TestWS09_UsageLogging drives successful, usage-less and failing calls and
// checks the persisted logs field by field, then filters, pagination,
// detail and summary against exactly those logs.
func TestWS09_UsageLogging(t *testing.T) {
	e := ws09NewEnv(t, "log")
	ok, nousage, e500, e400 := e.Models["ok"], e.Models["nousage"], e.Models["e500"], e.Models["e400"]

	before := time.Now().UTC().Add(-2 * time.Second)
	for i := 0; i < 3; i++ {
		r := e.chat(t, "ok")
		if r.Status != 200 {
			t.Fatalf("chat ok #%d: %d %s", i, r.Status, r.Body)
		}
		time.Sleep(20 * time.Millisecond) // distinct created_at for ordering
	}
	if r := e.chat(t, "nousage"); r.Status != 200 {
		t.Fatalf("chat nousage: %d %s", r.Status, r.Body)
	}
	if r := e.chat(t, "e500"); r.Status != http.StatusBadGateway {
		t.Fatalf("chat e500: want 502, got %d %s", r.Status, r.Body)
	}
	if r := e.chat(t, "e400"); r.Status != http.StatusBadGateway {
		t.Fatalf("chat e400: want 502, got %d %s", r.Status, r.Body)
	}
	after := time.Now().UTC().Add(2 * time.Second)

	okLogs := ws09WaitLogs(t, ok.Name, 3)
	nuLogs := ws09WaitLogs(t, nousage.Name, 1)
	e5Logs := ws09WaitLogs(t, e500.Name, 1)
	e4Logs := ws09WaitLogs(t, e400.Name, 1)

	t.Run("success log fields and cost arithmetic", func(t *testing.T) {
		wantIn, wantOut := ws09Cost(5, 4)
		for _, l := range okLogs {
			if l.RequestedModel != ok.Name || l.UsedModel != "fake-ok" {
				t.Errorf("models: requested=%q used=%q", l.RequestedModel, l.UsedModel)
			}
			if l.ProviderID != e.ProvID || l.KeyID != e.KeyID || l.MappingID != ok.MappingID {
				t.Errorf("routing ids: provider=%s key=%s mapping=%s; want %s %s %s", l.ProviderID, l.KeyID, l.MappingID, e.ProvID, e.KeyID, ok.MappingID)
			}
			if l.PromptTokens != 5 || l.CompletionTokens != 4 || l.TotalTokens != 9 || l.CachedTokens != 0 {
				t.Errorf("tokens: %d/%d/%d cached %d", l.PromptTokens, l.CompletionTokens, l.TotalTokens, l.CachedTokens)
			}
			if !ws09Approx(l.InputCost, wantIn) || !ws09Approx(l.OutputCost, wantOut) || !ws09Approx(l.TotalCost, wantIn+wantOut) {
				t.Errorf("cost: in=%v out=%v total=%v; want %v %v %v", l.InputCost, l.OutputCost, l.TotalCost, wantIn, wantOut, wantIn+wantOut)
			}
			if l.StatusCode != 200 || l.HasError || l.ErrorMessage != "" || l.Streamed || l.IsFallback || l.FinishReason != "stop" {
				t.Errorf("metadata: %+v", l)
			}
			if l.CreatedAt.Before(before) || l.CreatedAt.After(after) {
				t.Errorf("created_at %s outside [%s, %s]", l.CreatedAt, before, after)
			}
			if l.DurationMs < 0 {
				t.Errorf("negative duration %d", l.DurationMs)
			}
		}
		for i := 1; i < len(okLogs); i++ {
			if okLogs[i-1].CreatedAt.Before(okLogs[i].CreatedAt) {
				t.Errorf("list not ordered newest first: %s before %s", okLogs[i-1].CreatedAt, okLogs[i].CreatedAt)
			}
		}
	})

	t.Run("no usage block logs zero tokens and zero cost", func(t *testing.T) {
		l := nuLogs[0]
		if l.PromptTokens != 0 || l.CompletionTokens != 0 || l.TotalTokens != 0 || l.TotalCost != 0 || l.InputCost != 0 || l.OutputCost != 0 {
			t.Errorf("nousage log: %+v", l)
		}
		if l.HasError || l.StatusCode != 200 {
			t.Errorf("nousage should be a success: %+v", l)
		}
	})

	t.Run("failed requests are logged with error status", func(t *testing.T) {
		for _, c := range []struct {
			l    ws09Log
			code int
		}{{e5Logs[0], 500}, {e4Logs[0], 400}} {
			l := c.l
			if !l.HasError || l.StatusCode != c.code {
				t.Errorf("%s: has_error=%v status=%d, want true/%d", l.RequestedModel, l.HasError, l.StatusCode, c.code)
			}
			if !strings.Contains(l.ErrorMessage, fmt.Sprintf("upstream returned %d", c.code)) {
				t.Errorf("%s: error_message %q", l.RequestedModel, l.ErrorMessage)
			}
			if l.TotalTokens != 0 || l.TotalCost != 0 {
				t.Errorf("%s: failed call billed: %+v", l.RequestedModel, l)
			}
			if strings.Contains(l.ErrorMessage, e.Token) {
				t.Errorf("error_message leaks provider token")
			}
		}
	})

	t.Run("detail by id", func(t *testing.T) {
		admin := AdminAPI(t)
		for _, want := range []ws09Log{okLogs[0], e5Logs[0]} {
			r := admin.Get(t, "/usage/"+want.ID)
			if r.Status != 200 {
				t.Fatalf("GET /usage/%s: %d %s", want.ID, r.Status, r.Body)
			}
			var got ws09Log
			r.JSON(t, &got)
			if got.ID != want.ID || got.RequestedModel != want.RequestedModel || got.TotalCost != want.TotalCost ||
				got.StatusCode != want.StatusCode || got.ErrorMessage != want.ErrorMessage || !got.CreatedAt.Equal(want.CreatedAt) {
				t.Errorf("detail differs from list item:\n got %+v\nwant %+v", got, want)
			}
		}
		ws09WantErr(t, admin.Get(t, "/usage/7d4b2f0e-0000-4000-8000-000000000909"), 404, "usage log not found", "unknown uuid")
		ws09WantErr(t, admin.Get(t, "/usage/not-a-uuid"), 400, "invalid usage log id", "malformed id")
	})

	t.Run("filter by model is exact", func(t *testing.T) {
		// prefix of our model names must not match anything of ours
		p := ws09Usage(t, "model="+url.QueryEscape(e.Seg))
		if p.Page.Total != 0 {
			t.Errorf("model=%s (prefix) matched %d logs; filter should be exact", e.Seg, p.Page.Total)
		}
		p = ws09Usage(t, "model="+url.QueryEscape(strings.ToUpper(ok.Name)))
		if p.Page.Total != 0 {
			t.Errorf("upper-cased model name matched %d logs", p.Page.Total)
		}
	})

	t.Run("filter by status (has_error)", func(t *testing.T) {
		if p := ws09Usage(t, "has_error=true&model="+url.QueryEscape(ok.Name)); p.Page.Total != 0 {
			t.Errorf("ok model has_error=true: %d", p.Page.Total)
		}
		if p := ws09Usage(t, "has_error=false&model="+url.QueryEscape(ok.Name)); p.Page.Total != 3 {
			t.Errorf("ok model has_error=false: %d, want 3", p.Page.Total)
		}
		if p := ws09Usage(t, "has_error=true&model="+url.QueryEscape(e500.Name)); p.Page.Total != 1 || !p.Items[0].HasError {
			t.Errorf("e500 has_error=true: %+v", p.Page)
		}
		if p := ws09Usage(t, "has_error=false&model="+url.QueryEscape(e500.Name)); p.Page.Total != 0 {
			t.Errorf("e500 has_error=false: %d", p.Page.Total)
		}
		// unrecognised values are ignored (no filter) rather than rejected
		if p := ws09Usage(t, "has_error=yes&model="+url.QueryEscape(e500.Name)); p.Page.Total != 1 {
			t.Errorf("has_error=yes: %d, want 1 (ignored)", p.Page.Total)
		}
	})

	t.Run("filter by provider", func(t *testing.T) {
		p := ws09Usage(t, "limit=100&provider="+e.ProvID)
		if p.Page.Total != 6 {
			t.Errorf("provider=%s: total %d, want 6 (3 ok + nousage + 500 + 400)", e.ProvID, p.Page.Total)
		}
		for _, l := range p.Items {
			if l.ProviderID != e.ProvID {
				t.Errorf("foreign provider in result: %s", l.ProviderID)
			}
		}
	})

	t.Run("filter by provider rejects malformed id with 400", func(t *testing.T) {
		r := AdminAPI(t).Get(t, "/usage?provider=not-a-uuid")
		// FINDING WS09-1: a malformed provider id reaches Postgres and returns 500.
		ws09WantErr(t, r, 400, "", "provider=not-a-uuid")
	})

	t.Run("filter by date range", func(t *testing.T) {
		m := "model=" + url.QueryEscape(ok.Name)
		rfc := func(tt time.Time) string { return url.QueryEscape(tt.Format(time.RFC3339Nano)) }
		if p := ws09Usage(t, m+"&from="+rfc(before)+"&to="+rfc(after)); p.Page.Total != 3 {
			t.Errorf("window around calls: %d, want 3", p.Page.Total)
		}
		if p := ws09Usage(t, m+"&to="+rfc(before)); p.Page.Total != 0 {
			t.Errorf("to before calls: %d, want 0", p.Page.Total)
		}
		if p := ws09Usage(t, m+"&from="+rfc(after)); p.Page.Total != 0 {
			t.Errorf("from after calls: %d, want 0", p.Page.Total)
		}
		if p := ws09Usage(t, m+"&from="+rfc(after)+"&to="+rfc(before)); p.Page.Total != 0 {
			t.Errorf("inverted range: %d, want 0", p.Page.Total)
		}
		// boundaries are inclusive: from == to == created_at of the middle log
		mid := url.QueryEscape(okLogs[1].CreatedAt.Format(time.RFC3339Nano))
		if p := ws09Usage(t, m+"&from="+mid+"&to="+mid); p.Page.Total != 1 || p.Items[0].ID != okLogs[1].ID {
			t.Errorf("exact instant: %+v", p.Page)
		}
		// offset timezone is honoured
		loc := time.FixedZone("x", -5*3600)
		if p := ws09Usage(t, m+"&from="+rfc(before.In(loc))+"&to="+rfc(after.In(loc))); p.Page.Total != 3 {
			t.Errorf("same window in -05:00: %d, want 3", p.Page.Total)
		}
		admin := AdminAPI(t)
		for _, q := range []string{"from=2026-01-01", "from=yesterday", "to=1700000000", "to=2026-13-01T00:00:00Z"} {
			ws09WantErr(t, admin.Get(t, "/usage?"+q), 400, "use RFC3339", q)
		}
	})

	t.Run("pagination limits and offsets", func(t *testing.T) {
		m := "model=" + url.QueryEscape(ok.Name)
		seen := map[string]bool{}
		for off := 0; off < 3; off++ {
			p := ws09Usage(t, fmt.Sprintf("%s&limit=1&offset=%d", m, off))
			if len(p.Items) != 1 || p.Page.Total != 3 || p.Page.Limit != 1 || p.Page.Offset != off {
				t.Fatalf("limit=1 offset=%d: %d items page=%+v", off, len(p.Items), p.Page)
			}
			if p.Items[0].ID != okLogs[off].ID {
				t.Errorf("offset %d: got %s want %s", off, p.Items[0].ID, okLogs[off].ID)
			}
			seen[p.Items[0].ID] = true
		}
		if len(seen) != 3 {
			t.Errorf("pages overlap: %v", seen)
		}
		if p := ws09Usage(t, m+"&limit=2&offset=2"); len(p.Items) != 1 || p.Page.Total != 3 {
			t.Errorf("last partial page: %d items", len(p.Items))
		}
		if p := ws09Usage(t, m+"&offset=3"); len(p.Items) != 0 || p.Page.Total != 3 || p.Items == nil {
			t.Errorf("offset past end: items=%v total=%d (want [] / 3)", p.Items, p.Page.Total)
		}
		// edge values are normalised, not rejected
		cases := []struct {
			q             string
			limit, offset int
		}{
			{"limit=0", 20, 0},
			{"limit=-5", 20, 0},
			{"limit=abc", 20, 0},
			{"limit=100", 100, 0},
			{"limit=101", 100, 0},
			{"limit=1000000", 100, 0},
			{"limit=99999999999999999999", 100, 0}, // overflow → Atoi clamps to MaxInt64 → capped
			{"offset=-1", 20, 0},
			{"offset=abc", 20, 0},
		}
		for _, c := range cases {
			p := ws09Usage(t, m+"&"+c.q)
			if p.Page.Limit != c.limit || p.Page.Offset != c.offset || p.Page.Total != 3 {
				t.Errorf("%s: page=%+v want limit=%d offset=%d total=3", c.q, p.Page, c.limit, c.offset)
			}
		}
		if p := ws09Usage(t, m+"&offset=99999999999999999999"); len(p.Items) != 0 || p.Page.Total != 3 || p.Page.Offset != math.MaxInt64 {
			t.Errorf("max int64 offset: %d items total %d", len(p.Items), p.Page.Total)
		}
	})

	t.Run("summary by_model equals sum of our logs", func(t *testing.T) {
		s := ws09Summ(t, "")
		for _, c := range []struct {
			name string
			logs []ws09Log
		}{{ok.Name, okLogs}, {nousage.Name, nuLogs}, {e500.Name, e5Logs}, {e400.Name, e4Logs}} {
			var want ws09ModelSummary
			for _, l := range c.logs {
				want.TotalRequests++
				want.TotalTokens += l.TotalTokens
				want.PromptTokens += l.PromptTokens
				want.CompletionTokens += l.CompletionTokens
				want.TotalCost += l.TotalCost
			}
			got, found := s.model(c.name)
			if !found {
				t.Errorf("summary has no by_model entry for %s", c.name)
				continue
			}
			if got.TotalRequests != want.TotalRequests || got.TotalTokens != want.TotalTokens || got.PromptTokens != want.PromptTokens ||
				got.CompletionTokens != want.CompletionTokens || !ws09Approx(got.TotalCost, want.TotalCost) {
				t.Errorf("%s: summary %+v != sum of logs %+v", c.name, got, want)
			}
		}
		okSum, _ := s.model(ok.Name)
		if okSum.TotalTokens != 27 || !ws09Approx(okSum.TotalCost, 3*(15+28)/1e6) {
			t.Errorf("ok model: 3 calls × (5×$3 + 4×$7)/1M expected 27 tokens / $0.000129, got %+v", okSum)
		}
		if s.Summary.TotalRequests < 6 || s.Summary.ErrorCount < 2 {
			t.Errorf("global summary smaller than our own contribution: %+v", s.Summary)
		}
		// windowed summary: our models are present inside the window and absent outside it
		in := ws09Summ(t, "from="+url.QueryEscape(before.Format(time.RFC3339Nano))+"&to="+url.QueryEscape(after.Format(time.RFC3339Nano)))
		if m, f := in.model(ok.Name); !f || m.TotalRequests != 3 {
			t.Errorf("windowed summary for ok model: %+v found=%v", m, f)
		}
		if d := in.PeriodStart.Sub(before); d > time.Microsecond || d < -time.Microsecond {
			t.Errorf("period_start %s != from %s", in.PeriodStart, before)
		}
		out := ws09Summ(t, "to="+url.QueryEscape(before.Format(time.RFC3339Nano)))
		if _, f := out.model(ok.Name); f {
			t.Errorf("summary with to<calls still lists %s", ok.Name)
		}
		fut := ws09Summ(t, "from="+url.QueryEscape(time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339)))
		if fut.Summary.TotalRequests != 0 || len(fut.ByModel) != 0 || fut.ByModel == nil {
			t.Errorf("future window should be empty with by_model=[]: %+v", fut)
		}
		ws09WantErr(t, AdminAPI(t).Get(t, "/usage/summary?from=2026-01-01"), 400, "use RFC3339", "summary from=date-only")
		ws09WantErr(t, AdminAPI(t).Get(t, "/usage/summary?to=nope"), 400, "use RFC3339", "summary to=nope")
	})
}

// TestWS09_UsageStreamedTokens checks that a streamed completion is logged
// with the tokens/cost reported in the stream's final usage chunk.
func TestWS09_UsageStreamedTokens(t *testing.T) {
	e := ws09NewEnv(t, "strm")
	ok := e.Models["ok"]
	res := e.GW.Stream(t, http.MethodPost, "/chat/completions", Chat(ok.Name, "ws09 stream "+Uniq("s"), true))
	buf := new(strings.Builder)
	b := make([]byte, 4096)
	for {
		n, err := res.Body.Read(b)
		buf.Write(b[:n])
		if err != nil {
			break
		}
	}
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(buf.String(), `"prompt_tokens":5`) {
		t.Fatalf("stream: %d %s", res.StatusCode, buf.String())
	}
	l := ws09WaitLogs(t, ok.Name, 1)[0]
	if !l.Streamed || l.HasError || l.StatusCode != 200 {
		t.Fatalf("streamed log metadata: %+v", l)
	}
	wantIn, wantOut := ws09Cost(5, 4)
	// FINDING WS09-2: streamed /v1/chat/completions are logged with 0 tokens and $0.
	if l.PromptTokens != 5 || l.CompletionTokens != 4 || !ws09Approx(l.TotalCost, wantIn+wantOut) {
		t.Errorf("streamed usage not recorded: upstream sent prompt 5 / completion 4 (cost %v) but log has %d/%d cost %v",
			wantIn+wantOut, l.PromptTokens, l.CompletionTokens, l.TotalCost)
	}
}

// TestWS09_UsageSummaryDefaultPeriod checks that a summary without from/to
// covers the period it reports. A log row aged 60 days is inserted directly
// (own model name, removed afterwards) since there is no API to backdate.
func TestWS09_UsageSummaryDefaultPeriod(t *testing.T) {
	f := FX(t)
	db, err := sql.Open("postgres", f.URLs.DB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() }) // registered first → runs after the row delete below
	model := Uniq("ws09-aged")
	id := "9a9a9a9a-0000-4000-8000-" + fmt.Sprintf("%012d", time.Now().UnixNano()%1_000_000_000_000)
	created := time.Now().UTC().AddDate(0, 0, -60)
	if _, err := db.Exec(`INSERT INTO usage_logs (id, key_id, requested_model, used_model, provider_id, mapping_id,
		prompt_tokens, completion_tokens, total_tokens, total_cost, created_at)
		VALUES ($1, gen_random_uuid(), $2, 'fake-ok', gen_random_uuid(), gen_random_uuid(), 5, 4, 9, 0.000043, $3)`,
		id, model, created); err != nil {
		t.Fatalf("insert aged log: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM usage_logs WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup aged row: %v", err)
		}
	})

	// list/detail/date filters see the aged row
	p := ws09Usage(t, "model="+url.QueryEscape(model))
	if p.Page.Total != 1 || p.Items[0].ID != id {
		t.Fatalf("aged row not listed: %+v", p)
	}
	from := url.QueryEscape(created.Add(-time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(created.Add(time.Hour).Format(time.RFC3339))
	if p := ws09Usage(t, "model="+url.QueryEscape(model)+"&from="+from+"&to="+to); p.Page.Total != 1 {
		t.Errorf("aged row not in its own window")
	}
	if p := ws09Usage(t, "model="+url.QueryEscape(model)+"&from="+url.QueryEscape(time.Now().AddDate(0, 0, -30).Format(time.RFC3339))); p.Page.Total != 0 {
		t.Errorf("aged row inside last-30-days window")
	}

	s := ws09Summ(t, "")
	if time.Since(s.PeriodStart) < 25*24*time.Hour || time.Since(s.PeriodStart) > 32*24*time.Hour {
		t.Fatalf("default period_start %s is not ~1 month ago", s.PeriodStart)
	}
	// FINDING WS09-3: summary without from/to aggregates all time but reports a 1-month period.
	if m, found := s.model(model); found && created.Before(s.PeriodStart) {
		t.Errorf("summary reports period %s..%s but includes %s (%d req) created %s, outside that period",
			s.PeriodStart.Format(time.RFC3339), s.PeriodEnd.Format(time.RFC3339), model, m.TotalRequests, created.Format(time.RFC3339))
	}
}

// TestWS09_Retention exercises the global retention config. It is a singleton,
// so the original state is restored in cleanup. The purge worker itself runs
// hourly (bootstrap.initServer: StartPurgeWorker(time.Hour); first run one
// hour after boot) and is not exercised here.
func TestWS09_Retention(t *testing.T) {
	admin := AdminAPI(t)
	orig := admin.Get(t, "/usage/retention")
	if orig.Status != 200 && orig.Status != 404 {
		t.Fatalf("GET retention: %d %s", orig.Status, orig.Body)
	}
	t.Cleanup(func() {
		if orig.Status == 404 {
			admin.Delete(t, "/usage/retention")
			return
		}
		o := orig.Map(t)
		admin.Put(t, "/usage/retention", map[string]any{
			"retention_days": o["retention_days"], "retain_messages": o["retain_messages"], "retain_response_body": o["retain_response_body"],
		})
	})

	type cfg struct {
		ID                 string `json:"id"`
		RetentionDays      int    `json:"retention_days"`
		RetainMessages     bool   `json:"retain_messages"`
		RetainResponseBody bool   `json:"retain_response_body"`
	}
	put := func(body any) (Resp, cfg) {
		r := admin.Put(t, "/usage/retention", body)
		var c cfg
		if r.Status == 200 {
			r.JSON(t, &c)
		}
		return r, c
	}
	get := func() cfg {
		r := admin.Get(t, "/usage/retention")
		if r.Status != 200 {
			t.Fatalf("GET retention: %d %s", r.Status, r.Body)
		}
		var c cfg
		r.JSON(t, &c)
		return c
	}

	t.Run("clear then get is 404, delete is idempotent", func(t *testing.T) {
		if r := admin.Delete(t, "/usage/retention"); r.Status != 204 {
			t.Fatalf("DELETE: %d %s", r.Status, r.Body)
		}
		ws09WantErr(t, admin.Get(t, "/usage/retention"), 404, "retention config not found", "GET after delete")
		if r := admin.Delete(t, "/usage/retention"); r.Status != 204 {
			t.Errorf("second DELETE: %d", r.Status)
		}
	})

	t.Run("create applies defaults for omitted fields", func(t *testing.T) {
		r, c := put(map[string]any{"retention_days": 30})
		if r.Status != 200 || c.RetentionDays != 30 || !c.RetainMessages || !c.RetainResponseBody || c.ID == "" {
			t.Fatalf("PUT 30: %d %s", r.Status, r.Body)
		}
		if g := get(); g != c {
			t.Errorf("GET %+v != PUT %+v", g, c)
		}
	})

	t.Run("partial update keeps other fields and id", func(t *testing.T) {
		before := get()
		r, c := put(map[string]any{"retain_messages": false})
		if r.Status != 200 || c.RetentionDays != 30 || c.RetainMessages || !c.RetainResponseBody || c.ID != before.ID {
			t.Errorf("partial PUT: %d %s", r.Status, r.Body)
		}
		r, c = put(map[string]any{})
		if r.Status != 200 || c != get() || c.RetentionDays != 30 {
			t.Errorf("empty PUT: %d %s", r.Status, r.Body)
		}
	})

	t.Run("zero means retain forever", func(t *testing.T) {
		r, c := put(map[string]any{"retention_days": 0})
		if r.Status != 200 || c.RetentionDays != 0 || get().RetentionDays != 0 {
			t.Errorf("PUT 0: %d %s", r.Status, r.Body)
		}
	})

	t.Run("invalid values are rejected with 400", func(t *testing.T) {
		put(map[string]any{"retention_days": 7})
		for _, b := range []any{
			map[string]any{"retention_days": -1},
			map[string]any{"retention_days": math.MinInt32},
			`{"retention_days": "ten"}`,
			`{"retention_days": 1.5}`,
			`{"retain_messages": "yes"}`,
			`not json`,
		} {
			r, _ := put(b)
			if r.Status != 400 {
				t.Errorf("PUT %v: want 400, got %d %s", b, r.Status, r.Body)
			}
		}
		r, _ := put(map[string]any{"retention_days": -1})
		ws09WantErr(t, r, 400, "retention_days must be non-negative", "negative days message")
		if g := get(); g.RetentionDays != 7 {
			t.Errorf("rejected PUTs changed config: %+v", g)
		}
	})

	t.Run("huge values are rejected with 400, not 500", func(t *testing.T) {
		put(map[string]any{"retention_days": 7})
		// FINDING WS09-4: 2^31 overflows the INTEGER column → 500; 2147483647 is
		// accepted but makes the purge query overflow (timestamp out of range).
		for _, d := range []int64{2147483648, 1 << 40} {
			r, _ := put(map[string]any{"retention_days": d})
			if r.Status != 400 {
				t.Errorf("PUT retention_days=%d: want 400, got %d %s", d, r.Status, r.Body)
			}
		}
		r, _ := put(map[string]any{"retention_days": 2147483647})
		if r.Status != 400 {
			t.Errorf("PUT retention_days=2147483647: want 400 (purge cannot compute now()-%d days), got %d %s", 2147483647, r.Status, r.Body)
		}
		r, _ = put(map[string]any{"retention_days": 36500})
		if r.Status != 200 {
			t.Errorf("PUT 100 years: %d %s", r.Status, r.Body)
		}
	})

	t.Run("write requires usage:write", func(t *testing.T) {
		f := FX(t)
		gw := Bearer(f.URLs.API, f.Personas["gw_key"].Secret)
		ws09WantErr(t, gw.Put(t, "/usage/retention", map[string]any{"retention_days": 1}), 403, "", "gw_key PUT")
		ws09WantErr(t, gw.Delete(t, "/usage/retention"), 403, "", "gw_key DELETE")
		ws09WantErr(t, Anon(f.URLs.API).Put(t, "/usage/retention", map[string]any{"retention_days": 1}), 401, "", "anon PUT")
		if g := get(); g.RetentionDays == 1 {
			t.Errorf("unauthorized PUT took effect")
		}
	})
}

// TestWS09_UsageAuthz checks the read endpoints' auth (full matrix is WS-02).
func TestWS09_UsageAuthz(t *testing.T) {
	f := FX(t)
	for _, p := range []string{"/usage", "/usage/summary", "/usage/retention", "/usage/7d4b2f0e-0000-4000-8000-000000000909"} {
		ws09WantErr(t, Anon(f.URLs.API).Get(t, p), 401, "", "anon "+p)
		ws09WantErr(t, Bearer(f.URLs.API, f.Personas["gw_key"].Secret).Get(t, p), 403, "", "gw_key "+p)
		ws09WantErr(t, Bearer(f.URLs.API, f.Personas["noperm_key"].Secret).Get(t, p), 403, "", "noperm_key "+p)
	}
}
