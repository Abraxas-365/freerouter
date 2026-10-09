//go:build e2e

package api

// WS-07 helpers: own service account (own rate-limit + routing subject), own
// fakellm-backed catalog under /ws07/<seg>/..., fakellm request filtering, SSE
// reading and usage lookups. Everything created here is removed in t.Cleanup.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ws07GW is a gateway caller with its own IAMKit subject.
type ws07GW struct {
	*Client
	Subject string // service-account id == JWT subject used for rate limits, routing configs, cache keys
	Secret  string
}

// ws07NewGateway returns the shared WS-07 gateway caller (own service account
// and generous per-subject rate limit, so WS-07 load never touches gw_key).
//
// IAMKit rate-limits service-account creation and machine-token exchange
// (429 → FreeRouter 502/401) while other workers run, so WS-07 creates only
// two service accounts per run (primary + "other" for cross-subject checks)
// and reuses them. Tests are sequential; per-subject state (routing config,
// rate-limit overrides) is set and restored per test. TestWS07_ZZZ_Cleanup
// revokes them at the end of the run.
func ws07NewGateway(t *testing.T) ws07GW {
	t.Helper()
	return ws07Shared(t, 0)
}

// ws07OtherGateway is a second, distinct subject.
func ws07OtherGateway(t *testing.T) ws07GW {
	t.Helper()
	return ws07Shared(t, 1)
}

var (
	ws07Pool    [2]*ws07GW
	ws07PoolRL  [2]string
	ws07PoolMu  sync.Mutex
	ws07PoolRPM = 5000
	ws07PoolMC  = 100
)

func ws07Shared(t *testing.T, i int) ws07GW {
	t.Helper()
	ws07PoolMu.Lock()
	defer ws07PoolMu.Unlock()
	if ws07Pool[i] == nil {
		gw, rl := ws07CreateGateway(t)
		ws07Pool[i], ws07PoolRL[i] = &gw, rl
	}
	return *ws07Pool[i]
}

// ws07WithConcurrency temporarily sets the primary subject's max_concurrent.
func ws07WithConcurrency(t *testing.T, gw ws07GW, maxConcurrent int) {
	t.Helper()
	admin := AdminAPI(t)
	id := ws07PoolRL[0]
	if r := admin.Patch(t, "/rate-limits/"+id, map[string]any{"max_concurrent": maxConcurrent}); r.Status != 200 {
		t.Fatalf("patch rate limit: %d %s", r.Status, r.Body)
	}
	t.Cleanup(func() { admin.Patch(t, "/rate-limits/"+id, map[string]any{"max_concurrent": ws07PoolMC}) })
}

func ws07CreateGateway(t *testing.T) (ws07GW, string) {
	t.Helper()
	f := FX(t)
	admin := AdminAPI(t)
	var r Resp
	for i := 0; i < 12; i++ {
		r = admin.Post(t, "/service-accounts", map[string]any{
			"name": Uniq("ws07-gw"), "permissions": []string{"freerouter:gateway:invoke"},
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
		"name": Uniq("ws07-rl"), "subject_id": sa.ID, "rpm": ws07PoolRPM, "max_concurrent": ws07PoolMC,
	})
	if rl.Status != http.StatusCreated {
		t.Fatalf("create rate limit: %d %s", rl.Status, rl.Body)
	}
	gw := ws07GW{Client: Bearer(f.URLs.Gateway, sa.Secret), Subject: sa.ID, Secret: sa.Secret}
	// Warm the server's machine-token cache (exchange may be throttled → 401).
	for i := 0; i < 20; i++ {
		if g := gw.Get(t, "/models"); g.Status == 200 {
			return gw, rl.Map(t)["id"].(string)
		}
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
	t.Fatalf("new service account never authenticated (IAMKit machine-token throttled)")
	return gw, ""
}

// ws07CleanupPool revokes the shared service accounts and their rate limits.
func ws07CleanupPool(t *testing.T) {
	ws07PoolMu.Lock()
	defer ws07PoolMu.Unlock()
	admin := AdminAPI(t)
	for i, gw := range ws07Pool {
		if gw == nil {
			continue
		}
		admin.Delete(t, "/rate-limits/"+ws07PoolRL[i])
		ok := false
		for j := 0; j < 8 && !ok; j++ {
			d := admin.Delete(t, "/service-accounts/"+gw.Subject)
			ok = d.Status == http.StatusNoContent || d.Status == http.StatusNotFound
			if !ok {
				time.Sleep(time.Duration(j+1) * time.Second)
			}
		}
		if !ok {
			t.Errorf("could not revoke ws07 service account %s", gw.Subject)
		}
		ws07Pool[i] = nil
	}
}

// ws07SetStrategy creates a routing config for the subject.
func ws07SetStrategy(t *testing.T, subject, strategy string) {
	t.Helper()
	admin := AdminAPI(t)
	r := admin.Post(t, "/routing-configs", map[string]any{"subject_id": subject, "strategy": strategy})
	if r.Status != http.StatusCreated {
		t.Fatalf("create routing config: %d %s", r.Status, r.Body)
	}
	id := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/routing-configs/"+id) })
}

// ws07Prov is an own provider pointing at fakellm under /ws07/<seg>.
type ws07Prov struct {
	ID, KeyID, Seg, Token, Base string
}

var ws07ProtoSuffix = map[string]string{"openai": "/v1", "anthropic": "/v1", "google": "/v1beta", "cohere": "/v2"}

// ws07NewProvider creates a provider + one api_key key. seg must be unique.
// baseOverride (optional) replaces the fakellm base URL (e.g. a dead port).
func ws07NewProvider(t *testing.T, protocol, seg string, baseOverride ...string) ws07Prov {
	t.Helper()
	f := FX(t)
	admin := AdminAPI(t)
	base := f.URLs.FakeLLM + "/ws07/" + seg + ws07ProtoSuffix[protocol]
	if len(baseOverride) > 0 {
		base = baseOverride[0]
	}
	r := admin.Post(t, "/providers", map[string]any{
		"name": "ws07-" + seg, "protocol": protocol, "base_url": base, "description": "ws07", "streaming": true,
	})
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider: %d %s", r.Status, r.Body)
	}
	p := ws07Prov{ID: r.Map(t)["id"].(string), Seg: seg, Token: "ws07-tok-" + seg, Base: base}
	t.Cleanup(func() { admin.Delete(t, "/providers/"+p.ID) })
	p.KeyID = ws07AddKey(t, p.ID, "ws07-key-"+seg, p.Token, "")
	return p
}

// ws07AddKey adds a provider key; baseURL (optional) overrides the provider base URL.
func ws07AddKey(t *testing.T, providerID, name, token, baseURL string) string {
	t.Helper()
	body := map[string]any{"provider_id": providerID, "name": name, "key_type": "api_key", "token": token}
	if baseURL != "" {
		body["base_url"] = baseURL
	}
	r := AdminAPI(t).Post(t, "/provider-keys", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create provider key: %d %s", r.Status, r.Body)
	}
	return r.Map(t)["id"].(string)
}

// ws07NewModel creates a model and returns (id, name).
func ws07NewModel(t *testing.T, prefix string) (string, string) {
	t.Helper()
	admin := AdminAPI(t)
	name := Uniq("ws07-" + prefix)
	r := admin.Post(t, "/models", map[string]any{"name": name, "family": "ws07", "description": "ws07"})
	if r.Status != http.StatusCreated {
		t.Fatalf("create model: %d %s", r.Status, r.Body)
	}
	id := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/models/"+id) })
	return id, name
}

// ws07Map maps a model to a provider with an external id and prices; extra adds fields (capabilities, etc.).
func ws07Map(t *testing.T, modelID, providerID, external string, in, out float64, extra map[string]any) string {
	t.Helper()
	body := map[string]any{
		"model_id": modelID, "provider_id": providerID, "external_id": external,
		"input_price": in, "output_price": out, "streaming": true, "tools": true, "json_output": true,
	}
	for k, v := range extra {
		body[k] = v
	}
	r := AdminAPI(t).Post(t, "/mappings", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create mapping: %d %s", r.Status, r.Body)
	}
	return r.Map(t)["id"].(string)
}

// ws07Fallback links model → fallback model.
func ws07Fallback(t *testing.T, modelID, fallbackID string, priority int) {
	t.Helper()
	r := AdminAPI(t).Post(t, "/model-fallbacks", map[string]any{"model_id": modelID, "fallback_model_id": fallbackID, "priority": priority})
	if r.Status != http.StatusCreated {
		t.Fatalf("create fallback: %d %s", r.Status, r.Body)
	}
}

// ws07SimpleModel creates provider(seg) + model mapped to external id. Returns model name and provider.
func ws07SimpleModel(t *testing.T, protocol, seg, external string) (string, ws07Prov) {
	t.Helper()
	p := ws07NewProvider(t, protocol, seg)
	mid, name := ws07NewModel(t, seg)
	ws07Map(t, mid, p.ID, external, 1.0, 2.0, nil)
	return name, p
}

// ws07Rec is one request recorded by fakellm.
type ws07Rec struct {
	At      time.Time         `json:"at"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	Model   string            `json:"model"`
}

func (r ws07Rec) BodyMap(t *testing.T) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("recorded body is not an object: %s", r.Body)
	}
	return m
}

// ws07Upstream returns fakellm requests whose path starts with /ws07/<seg>/.
func ws07Upstream(t *testing.T, seg string) []ws07Rec {
	t.Helper()
	var all []ws07Rec
	Anon(FX(t).URLs.FakeLLM).Get(t, "/_requests").JSON(t, &all)
	var out []ws07Rec
	for _, r := range all {
		if strings.HasPrefix(r.Path, "/ws07/"+seg+"/") {
			out = append(out, r)
		}
	}
	return out
}

// ws07UpstreamWith returns fakellm requests (any path) whose body contains marker.
func ws07UpstreamWith(t *testing.T, marker string) []ws07Rec {
	t.Helper()
	var all []ws07Rec
	Anon(FX(t).URLs.FakeLLM).Get(t, "/_requests").JSON(t, &all)
	var out []ws07Rec
	for _, r := range all {
		if bytes.Contains(r.Body, []byte(marker)) {
			out = append(out, r)
		}
	}
	return out
}

// ws07SSE is a parsed server-sent-event stream.
type ws07SSE struct {
	Status  int
	Header  http.Header
	Events  []ws07Event
	Raw     string
	ReadErr error
	Elapsed time.Duration
}

type ws07Event struct {
	Event string
	Data  string
}

// DataJSON returns the non-[DONE] data payloads decoded.
func (s ws07SSE) DataJSON(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, e := range s.Events {
		if e.Data == "[DONE]" || e.Data == "" {
			continue
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(e.Data), &m); err != nil {
			t.Fatalf("SSE data is not JSON: %q", e.Data)
		}
		out = append(out, m)
	}
	return out
}

func (s ws07SSE) HasDone() bool {
	for _, e := range s.Events {
		if e.Data == "[DONE]" {
			return true
		}
	}
	return false
}

func (s ws07SSE) EventNames() []string {
	var out []string
	for _, e := range s.Events {
		out = append(out, e.Event)
	}
	return out
}

// ws07Stream POSTs a JSON body and reads the whole SSE response until EOF (bounded by timeout).
func ws07Stream(t *testing.T, c *Client, path string, body any, timeout time.Duration) ws07SSE {
	t.Helper()
	j, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+path, bytes.NewReader(j))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	out := ws07SSE{Status: res.StatusCode, Header: res.Header}
	var raw strings.Builder
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	cur := ws07Event{}
	for sc.Scan() {
		line := sc.Text()
		raw.WriteString(line + "\n")
		switch {
		case strings.HasPrefix(line, "event:"):
			cur.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			cur.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			if cur.Event != "" || cur.Data != "" {
				out.Events = append(out.Events, cur)
			}
			cur = ws07Event{}
		}
	}
	if cur.Event != "" || cur.Data != "" {
		out.Events = append(out.Events, cur)
	}
	out.ReadErr = sc.Err()
	out.Raw = raw.String()
	out.Elapsed = time.Since(start)
	return out
}

// ws07Usage lists usage logs for a requested model (newest first).
func ws07Usage(t *testing.T, model string) []map[string]any {
	t.Helper()
	var page struct {
		Items []map[string]any `json:"items"`
	}
	r := AdminAPI(t).Get(t, "/usage?limit=100&model="+model)
	if r.Status != 200 {
		t.Fatalf("usage list: %d %s", r.Status, r.Body)
	}
	r.JSON(t, &page)
	return page.Items
}

// ws07WaitUsage polls until at least n usage rows exist for model.
func ws07WaitUsage(t *testing.T, model string, n int) []map[string]any {
	t.Helper()
	var rows []map[string]any
	Eventually(t, 10*time.Second, func() bool {
		rows = ws07Usage(t, model)
		return len(rows) >= n
	}, fmt.Sprintf("%d usage rows for %s", n, model))
	return rows
}

// ws07Multipart builds a multipart body with fields + one file part.
func ws07Multipart(fields map[string]string, fileField, fileName string, content []byte) (io.Reader, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	fw, _ := w.CreateFormFile(fileField, fileName)
	_, _ = fw.Write(content)
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

// ws07ChatContent returns choices[0].message.content of an OpenAI chat response.
func ws07ChatContent(t *testing.T, m map[string]any) string {
	t.Helper()
	ch, _ := m["choices"].([]any)
	if len(ch) == 0 {
		t.Fatalf("no choices in %v", m)
	}
	msg, _ := ch[0].(map[string]any)["message"].(map[string]any)
	s, _ := msg["content"].(string)
	return s
}

// ws07StreamText concatenates choices[0].delta.content over OpenAI chunks.
func ws07StreamText(t *testing.T, chunks []map[string]any) string {
	t.Helper()
	var b strings.Builder
	for _, c := range chunks {
		ch, _ := c["choices"].([]any)
		if len(ch) == 0 {
			continue
		}
		d, _ := ch[0].(map[string]any)["delta"].(map[string]any)
		if s, ok := d["content"].(string); ok {
			b.WriteString(s)
		}
	}
	return b.String()
}

func ws07Num(v any) float64 {
	f, _ := v.(float64)
	return f
}

// ws07LocalReq is one request received by a test-local upstream.
type ws07LocalReq struct {
	At        time.Time
	Path      string
	Header    http.Header
	Body      []byte
	cancelled atomic.Bool
}

// ws07Local is an in-process fake upstream for behaviours fakellm cannot
// script (auth failures, latency, delayed SSE chunks, binary audio).
type ws07Local struct {
	URL  string
	mu   sync.Mutex
	reqs []*ws07LocalReq
}

func ws07NewLocal(t *testing.T, h func(w http.ResponseWriter, r *http.Request, rec *ws07LocalReq)) *ws07Local {
	t.Helper()
	l := &ws07Local{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec := &ws07LocalReq{At: time.Now(), Path: r.URL.Path, Header: r.Header.Clone(), Body: body}
		l.mu.Lock()
		l.reqs = append(l.reqs, rec)
		l.mu.Unlock()
		h(w, r, rec)
	}))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	l.URL = srv.URL
	return l
}

func (l *ws07Local) Reqs() []*ws07LocalReq {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]*ws07LocalReq(nil), l.reqs...)
}

func (l *ws07Local) Count() int { return len(l.Reqs()) }

// ws07WriteChatOK writes a minimal OpenAI chat completion.
func ws07WriteChatOK(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "chatcmpl-local", "object": "chat.completion", "created": time.Now().Unix(), "model": "local",
		"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": content}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
}

// ws07Webhook subscribes the sink path /hook/<name> to events; returns the sink path.
func ws07Webhook(t *testing.T, name string, events ...string) string {
	t.Helper()
	admin := AdminAPI(t)
	path := "/hook/" + name
	r := admin.Post(t, "/webhooks", map[string]any{"url": FX(t).URLs.WebhookSink + path, "events": events})
	if r.Status != http.StatusCreated {
		t.Fatalf("create webhook: %d %s", r.Status, r.Body)
	}
	id := r.Map(t)["id"].(string)
	t.Cleanup(func() { admin.Delete(t, "/webhooks/"+id) })
	return path
}

// ws07SinkBodies returns delivery bodies for an exact sink path.
func ws07SinkBodies(t *testing.T, path string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, d := range SinkDeliveries(t) {
		if d["path"] != path {
			continue
		}
		if b, ok := d["body"].(map[string]any); ok {
			out = append(out, b)
		}
	}
	return out
}
