//go:build e2e

package api

// WS-09 — /health, /metrics and read-only security/ops checks.
//
// Dependency-down behaviour is NOT exercised here (deferred to a serial run).
// From code (internal/bootstrap/container.go initServer): /health is a static
// handler returning {"status":"ok"} — it pings neither Postgres, Redis nor
// IAMKit, so it keeps answering 200 when those are down.

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWS09_Health(t *testing.T) {
	f := FX(t)
	for _, c := range []*Client{Anon(f.URLs.Server), Bearer(f.URLs.Server, "garbage"), Bearer(f.URLs.Server, f.Personas["expired_key"].Secret)} {
		r := c.Get(t, "/health")
		if r.Status != 200 {
			t.Fatalf("/health: %d %s", r.Status, r.Body)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("/health content-type %q", ct)
		}
		m := r.Map(t)
		if len(m) != 1 || m["status"] != "ok" {
			t.Errorf("/health body: %s", r.Body)
		}
		if r.Header.Get("X-Request-Id") == "" {
			t.Errorf("/health: no X-Request-Id")
		}
	}
	if r := Anon(f.URLs.Server).Do(t, http.MethodPost, "/health", nil); r.Status != http.StatusMethodNotAllowed && r.Status != http.StatusNotFound {
		t.Errorf("POST /health: %d", r.Status)
	}
	if r := Anon(f.URLs.Server).Do(t, http.MethodHead, "/health", nil); r.Status != 200 {
		t.Errorf("HEAD /health: %d", r.Status)
	}
}

// ws09Metric returns the value of the first sample whose name and labels match.
func ws09Metric(body, name string, labels map[string]string) (float64, bool) {
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, name+"{") && !strings.HasPrefix(line, name+" ") {
			continue
		}
		ok := true
		for k, v := range labels {
			if !strings.Contains(line, k+`="`+v+`"`) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		i := strings.LastIndex(line, " ")
		v, err := strconv.ParseFloat(line[i+1:], 64)
		if err != nil {
			continue
		}
		return v, true
	}
	return 0, false
}

func TestWS09_Metrics(t *testing.T) {
	f := FX(t)
	metrics := Bearer(f.URLs.Server, f.Personas["noperm_key"].Secret)

	t.Run("auth", func(t *testing.T) {
		ws09WantErr(t, Anon(f.URLs.Server).Get(t, "/metrics"), 401, "", "anon /metrics")
		ws09WantErr(t, Bearer(f.URLs.Server, "not-a-token").Get(t, "/metrics"), 401, "", "garbage token /metrics")
		ws09WantErr(t, Bearer(f.URLs.Server, f.Personas["expired_key"].Secret).Get(t, "/metrics"), 401, "", "expired key /metrics")
		ws09WantErr(t, Bearer(f.URLs.Server, f.Personas["gw_key"].Secret).Get(t, "/metrics"), 403, "insufficient permissions", "gw_key /metrics")
		if r := Bearer(f.URLs.Server, f.Personas["admin_key"].Secret).Get(t, "/metrics"); r.Status != 200 {
			t.Errorf("admin_key /metrics: %d", r.Status)
		}
	})

	t.Run("prometheus text format", func(t *testing.T) {
		r := metrics.Get(t, "/metrics")
		if r.Status != 200 {
			t.Fatalf("/metrics with metrics:read: %d %s", r.Status, r.Body)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
			t.Errorf("content-type %q", ct)
		}
		body := string(r.Body)
		for _, want := range []string{
			"# TYPE freerouter_gateway_requests_total counter",
			"# TYPE freerouter_gateway_request_duration_seconds histogram",
			"# TYPE freerouter_gateway_tokens_total counter",
			"# TYPE freerouter_gateway_errors_total counter",
			"# TYPE freerouter_gateway_in_flight_requests gauge",
			"# HELP freerouter_gateway_requests_total ",
			"# TYPE go_goroutines gauge",
			"# TYPE process_cpu_seconds_total counter",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("metrics output lacks %q", want)
			}
		}
		sample := regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*(\{[^}]*\})? ([-+0-9.eE]+|NaN|[+-]Inf)$`)
		for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}
			if !sample.MatchString(line) {
				t.Errorf("not a prometheus sample line: %q", line)
				break
			}
		}
	})

	t.Run("counters move after own gateway calls", func(t *testing.T) {
		e := ws09NewEnv(t, "met")
		ok, e500 := e.Models["ok"], e.Models["e500"]
		lOK := map[string]string{"model": ok.Name, "provider": e.ProvID, "protocol": "openai", "status": "ok"}
		lErr := map[string]string{"model": e500.Name, "provider": e.ProvID, "status": "error"}
		body0 := string(metrics.Get(t, "/metrics").Body)
		if _, found := ws09Metric(body0, "freerouter_gateway_requests_total", lOK); found {
			t.Fatalf("fresh model already has a counter")
		}
		for i := 0; i < 2; i++ {
			if r := e.chat(t, "ok"); r.Status != 200 {
				t.Fatalf("chat: %d %s", r.Status, r.Body)
			}
		}
		if r := e.chat(t, "e500"); r.Status != http.StatusBadGateway {
			t.Fatalf("chat e500: %d %s", r.Status, r.Body)
		}
		body := string(metrics.Get(t, "/metrics").Body)
		if v, _ := ws09Metric(body, "freerouter_gateway_requests_total", lOK); v != 2 {
			t.Errorf("requests_total{model=%s,status=ok} = %v, want 2", ok.Name, v)
		}
		if v, _ := ws09Metric(body, "freerouter_gateway_tokens_total", map[string]string{"model": ok.Name, "type": "prompt"}); v != 10 {
			t.Errorf("tokens_total prompt = %v, want 10", v)
		}
		if v, _ := ws09Metric(body, "freerouter_gateway_tokens_total", map[string]string{"model": ok.Name, "type": "completion"}); v != 8 {
			t.Errorf("tokens_total completion = %v, want 8", v)
		}
		if v, _ := ws09Metric(body, "freerouter_gateway_request_duration_seconds_count", map[string]string{"model": ok.Name}); v != 2 {
			t.Errorf("duration histogram count = %v, want 2", v)
		}
		if v, _ := ws09Metric(body, "freerouter_gateway_errors_total", map[string]string{"provider": e.ProvID, "status_code": "500"}); v != 1 {
			t.Errorf("errors_total{provider,500} = %v, want 1", v)
		}
		if v, _ := ws09Metric(body, "freerouter_gateway_retries_total", map[string]string{"provider": e.ProvID, "reason": "http_500"}); v != 1 {
			t.Errorf("retries_total{provider,http_500} = %v, want 1", v)
		}
		// FINDING WS09-5: a request that exhausts all routes on a retryable
		// error (500/429/503) is never counted in requests_total{status="error"}.
		if v, found := ws09Metric(body, "freerouter_gateway_requests_total", lErr); !found || v != 1 {
			t.Errorf("requests_total{model=%s,status=error} = %v (present=%v), want 1 — failed request not counted", e500.Name, v, found)
		}
	})
}

// TestWS09_CORS records the CORS policy (read-only).
func TestWS09_CORS(t *testing.T) {
	f := FX(t)
	pre := func(path string) *http.Response {
		req, _ := http.NewRequest(http.MethodOptions, f.URLs.Server+path, nil)
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Access-Control-Request-Method", "DELETE")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	for _, p := range []string{"/api/v1/providers", "/v1/chat/completions", "/metrics"} {
		res := pre(p)
		if res.StatusCode != 204 {
			t.Errorf("preflight %s: %d", p, res.StatusCode)
		}
		// Credentials must never be combined with a wildcard / reflected origin.
		if res.Header.Get("Access-Control-Allow-Credentials") == "true" {
			t.Errorf("preflight %s allows credentials for arbitrary origin", p)
		}
		if !strings.Contains(res.Header.Get("Access-Control-Allow-Headers"), "Authorization") {
			t.Errorf("preflight %s: Authorization not allowed (%q)", p, res.Header.Get("Access-Control-Allow-Headers"))
		}
	}
	// FINDING WS09-6: any origin may call the admin API with a bearer token.
	if got := pre("/api/v1/providers").Header.Get("Access-Control-Allow-Origin"); got == "*" || got == "https://evil.example" {
		t.Errorf("admin API preflight from https://evil.example: Access-Control-Allow-Origin=%q (want console origin only)", got)
	}
}

// TestWS09_SecurityHeaders records which hardening headers are sent.
func TestWS09_SecurityHeaders(t *testing.T) {
	f := FX(t)
	r := AdminAPI(t).Get(t, "/providers?limit=1")
	if r.Status != 200 {
		t.Fatalf("providers: %d", r.Status)
	}
	if r.Header.Get("X-Request-Id") == "" {
		t.Errorf("no X-Request-Id")
	}
	if s := r.Header.Get("Server"); s != "" {
		t.Logf("Server header disclosed: %q", s)
	}
	// FINDING WS09-7: none of the standard hardening headers are set.
	var missing []string
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Cache-Control"} {
		if r.Header.Get(h) == "" {
			missing = append(missing, h)
		}
	}
	if len(missing) > 0 {
		t.Errorf("admin API response (contains secrets-adjacent config) lacks headers: %v", missing)
	}
	// error responses must not leak internals
	r = AdminAPI(t).Get(t, "/usage?provider=x")
	if strings.Contains(string(r.Body), "pq:") || strings.Contains(string(r.Body), "SELECT") {
		t.Errorf("error body leaks SQL/driver details: %s", r.Body)
	}
	_ = f
}

// TestWS09_SecretsNotLogged greps the shared server log for this test's own
// secrets (provider token and gateway service-account secret) after using them.
func TestWS09_SecretsNotLogged(t *testing.T) {
	e := ws09NewEnv(t, "log")
	if r := e.chat(t, "ok"); r.Status != 200 {
		t.Fatalf("chat: %d %s", r.Status, r.Body)
	}
	if r := e.chat(t, "e400"); r.Status != http.StatusBadGateway {
		t.Fatalf("chat e400: %d %s", r.Status, r.Body)
	}
	// a request with a bogus bearer that looks like a secret
	bogus := "ik_svc_ws09bogus" + strconv.FormatInt(time.Now().UnixNano(), 36)
	Bearer(FX(t).URLs.Gateway, bogus).Post(t, "/chat/completions", Chat(e.Models["ok"].Name, "x", false))
	time.Sleep(500 * time.Millisecond)

	_, file, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", ".run", "server.log"))
	if err != nil {
		t.Skipf("server log not readable: %v", err)
	}
	log := string(b)
	for name, s := range map[string]string{"provider token": e.Token, "gateway SA secret": e.SASecret, "bogus bearer": bogus} {
		if strings.Contains(log, s) {
			t.Errorf("server.log contains %s", name)
		}
	}
	if strings.Contains(log, "Bearer "+e.SASecret[:min(len(e.SASecret), 16)]) {
		t.Errorf("server.log contains an Authorization header value")
	}
	// own upstream requests: provider token is only sent upstream, never echoed back to the client
	var recs []map[string]any
	Anon(FX(t).URLs.FakeLLM).Get(t, "/_requests").JSON(t, &recs)
	seen := false
	for _, r := range recs {
		if p, _ := r["path"].(string); strings.HasPrefix(p, "/ws09/"+e.Seg+"/") {
			h, _ := r["headers"].(map[string]any)
			if a, _ := h["Authorization"].(string); a == "Bearer "+e.Token {
				seen = true
			}
		}
	}
	if !seen {
		t.Errorf("fakellm never saw the provider token on /ws09/%s/ (cannot confirm the token is the one we grep for)", e.Seg)
	}
}
