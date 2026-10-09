//go:build e2e

package api

// WS-12 Part B — dependency outages against the live e2e stack.
//
// Stops iamkit / postgres / redis (docker compose project frv2e2e) one at a
// time, measures /health, a gateway call, an admin call and an access call,
// then starts the container again and asserts FreeRouter recovers WITHOUT a
// server restart. Containers are always restarted in t.Cleanup.
//
// DESTRUCTIVE for concurrent workers: run alone.
//   go test -tags e2e -count=1 -run 'TestWS12_Outage' ./e2e/api/

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	ws12Project     = "frv2e2e"
	ws12MaxLatency  = 10 * time.Second
	ws12RecoverWait = 30 * time.Second
)

func ws12Compose(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("docker", append([]string{"compose", "-p", ws12Project}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose %v: %v\n%s", args, err, out)
	}
}

func ws12Health(svc string) string {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.Health.Status}}", ws12Project+"-"+svc+"-1").Output()
	if err != nil {
		return "error"
	}
	return strings.TrimSpace(string(out))
}

func ws12WaitHealthy(t *testing.T, svc string, timeout time.Duration) time.Duration {
	t.Helper()
	start := time.Now()
	for time.Since(start) < timeout {
		if ws12Health(svc) == "healthy" {
			return time.Since(start)
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s not healthy after %s", svc, timeout)
	return 0
}

// ws12Outage stops svc and registers a cleanup that starts it again and waits
// for health, even if the test fails.
func ws12Outage(t *testing.T, svc string) {
	t.Helper()
	if h := ws12Health(svc); h != "healthy" {
		t.Fatalf("precondition: %s is %q, want healthy", svc, h)
	}
	t.Cleanup(func() {
		if ws12Health(svc) != "healthy" {
			ws12Compose(t, "start", svc)
			ws12WaitHealthy(t, svc, 120*time.Second)
		}
	})
	ws12Compose(t, "stop", svc)
}

type ws12Probe struct {
	Name    string
	Status  int
	Elapsed time.Duration
	Body    string
	Err     error
}

func (p ws12Probe) String() string {
	if p.Err != nil {
		return p.Name + ": ERR " + p.Err.Error() + " after " + p.Elapsed.Round(time.Millisecond).String()
	}
	b := p.Body
	if len(b) > 140 {
		b = b[:140] + "…"
	}
	return p.Name + ": " + http.StatusText(p.Status) + " (" + strconv.Itoa(p.Status) + ") in " + p.Elapsed.Round(time.Millisecond).String() + " " + b
}

var ws12HTTP = &http.Client{Timeout: 15 * time.Second}

func ws12Call(name string, c *Client, method, path string, body any) ws12Probe {
	cc := *c
	cc.HTTP = ws12HTTP
	start := time.Now()
	r, err := cc.DoCtx(context.Background(), method, path, body)
	return ws12Probe{Name: name, Status: r.Status, Elapsed: time.Since(start), Body: string(r.Body), Err: err}
}

// ws12Probes runs the standard probe set against the live server.
func ws12Probes(t *testing.T) map[string]ws12Probe {
	t.Helper()
	f := FX(t)
	gw := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret)
	admin := AdminAPI(t)
	out := map[string]ws12Probe{
		"health":  ws12Call("health", Anon(f.URLs.Server), "GET", "/health", nil),
		"gateway": ws12Call("gateway", gw, "POST", "/chat/completions", Chat("e2e-ok", Uniq("ws12 outage"), false)),
		"admin":   ws12Call("admin", admin, "GET", "/providers?limit=1", nil),
		"access":  ws12Call("access", admin, "GET", "/access/users?limit=1", nil),
	}
	for _, k := range []string{"health", "gateway", "admin", "access"} {
		t.Logf("  %s", out[k])
	}
	return out
}

// ws12AssertSane: no hang, no connection reset, JSON body on errors.
func ws12AssertSane(t *testing.T, p ws12Probe) {
	t.Helper()
	if p.Err != nil {
		var ne interface{ Timeout() bool }
		if errors.As(p.Err, &ne) && ne.Timeout() {
			t.Errorf("%s: hung (client timeout) after %s", p.Name, p.Elapsed)
		} else {
			t.Errorf("%s: transport error (reset/refused?) %v", p.Name, p.Err)
		}
		return
	}
	if p.Elapsed > ws12MaxLatency {
		t.Errorf("%s: took %s (> %s)", p.Name, p.Elapsed, ws12MaxLatency)
	}
	if p.Status >= 400 && !json.Valid([]byte(p.Body)) {
		t.Errorf("%s: %d with non-JSON body %q", p.Name, p.Status, p.Body)
	}
}

func ws12AssertRecovered(t *testing.T) {
	t.Helper()
	start := time.Now()
	f := FX(t)
	gw := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret)
	var last ws12Probe
	for time.Since(start) < ws12RecoverWait {
		last = ws12Call("gateway", gw, "POST", "/chat/completions", Chat("e2e-ok", Uniq("ws12 recover"), false))
		if last.Err == nil && last.Status == 200 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if last.Status != 200 {
		t.Fatalf("gateway did not recover within %s: %s", ws12RecoverWait, last)
	}
	t.Logf("recovered: gateway 200 %s after container healthy (call %s)", time.Since(start).Round(time.Millisecond), last.Elapsed.Round(time.Millisecond))
	for _, p := range []ws12Probe{
		ws12Call("admin", AdminAPI(t), "GET", "/providers?limit=1", nil),
		ws12Call("access", AdminAPI(t), "GET", "/access/users?limit=1", nil),
		ws12Call("service-accounts", AdminAPI(t), "GET", "/service-accounts?limit=1", nil),
	} {
		if p.Err != nil || p.Status != 200 {
			t.Errorf("after recovery %s", p)
		}
	}
}

// ws12ServerLogMark returns a function yielding server.log lines written since the mark.
func ws12ServerLogMark(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(ws12Root(), "e2e", ".run", "server.log")
	st, err := os.Stat(path)
	if err != nil {
		return func() string { return "" }
	}
	off := st.Size()
	return func() string {
		b, err := os.ReadFile(path)
		if err != nil || int64(len(b)) < off {
			return ""
		}
		return string(b[off:])
	}
}

func ws12AssertNoPanic(t *testing.T, log string) {
	t.Helper()
	if strings.Contains(log, "panic") {
		t.Errorf("server.log has a panic during the outage:\n%s", log)
	}
}

func TestWS12_Outage_IAMKit(t *testing.T) {
	logSince := ws12ServerLogMark(t)
	before := ws12Probes(t)
	for _, k := range []string{"gateway", "admin", "access"} {
		if before[k].Status != 200 {
			t.Fatalf("precondition %s", before[k])
		}
	}
	ws12Outage(t, "iamkit")
	t.Log("iamkit stopped")
	down := ws12Probes(t)
	for _, p := range down {
		ws12AssertSane(t, p)
	}
	if down["health"].Status != 200 {
		t.Errorf("/health with IAMKit down: %s (WS-09 expects static 200)", down["health"])
	} else {
		t.Logf("confirmed: /health stays 200 with IAMKit down (not a readiness probe)")
	}
	for _, k := range []string{"gateway", "admin", "access"} {
		if p := down[k]; p.Status != 401 && p.Status != 503 {
			t.Errorf("%s with IAMKit down: %d, want 401/503", k, p.Status)
		}
	}
	// Repeated calls must not accumulate latency (cached svc JWTs still need introspection).
	gw := Bearer(FX(t).URLs.Gateway, FX(t).Personas["gw_key"].Secret)
	for i := 0; i < 3; i++ {
		p := ws12Call("gateway-repeat", gw, "POST", "/chat/completions", Chat("e2e-ok", "ws12", false))
		ws12AssertSane(t, p)
	}

	ws12Compose(t, "start", "iamkit")
	t.Logf("iamkit healthy %s after start", ws12WaitHealthy(t, "iamkit", 120*time.Second).Round(time.Millisecond))
	ws12AssertRecovered(t)
	ws12AssertNoPanic(t, logSince())
}

func TestWS12_Outage_Postgres(t *testing.T) {
	logSince := ws12ServerLogMark(t)
	if p := ws12Probes(t)["gateway"]; p.Status != 200 {
		t.Fatalf("precondition %s", p)
	}
	ws12Outage(t, "postgres")
	t.Log("postgres stopped")
	down := ws12Probes(t)
	for _, p := range down {
		ws12AssertSane(t, p)
	}
	if down["health"].Status != 200 {
		t.Errorf("/health with Postgres down: %s", down["health"])
	} else {
		t.Logf("confirmed: /health stays 200 with Postgres down")
	}
	// IAMKit shares the Postgres instance, so authentication fails first.
	for _, k := range []string{"gateway", "admin"} {
		p := down[k]
		if p.Status == 200 {
			t.Errorf("%s succeeded with Postgres down: %s", k, p)
		}
		if p.Status == 401 {
			t.Logf("FINDING WS12-4: %s with Postgres down → 401 %q (IAMKit shares Postgres; outage reported as bad credentials, console logs users out)", k, p.Body)
		}
	}

	ws12Compose(t, "start", "postgres")
	t.Logf("postgres healthy %s after start", ws12WaitHealthy(t, "postgres", 120*time.Second).Round(time.Millisecond))
	ws12AssertRecovered(t)
	ws12AssertNoPanic(t, logSince())
}

func TestWS12_Outage_Redis(t *testing.T) {
	logSince := ws12ServerLogMark(t)
	before := ws12Probes(t)
	if before["gateway"].Status != 200 {
		t.Fatalf("precondition %s", before["gateway"])
	}
	ws12Outage(t, "redis")
	t.Log("redis stopped")
	down := ws12Probes(t)
	for _, p := range down {
		ws12AssertSane(t, p)
	}
	if down["health"].Status != 200 {
		t.Errorf("/health with Redis down: %s", down["health"])
	}
	for _, k := range []string{"gateway", "admin", "access"} {
		if down[k].Status != 200 {
			t.Errorf("%s with Redis down: %s, want 200 (Redis is optional: rate limit fails open, cache bypassed)", k, down[k])
		}
	}
	// Cache admin endpoint surfaces the outage as a JSON 5xx.
	c := ws12Call("cache-invalidate", AdminAPI(t), "DELETE", "/gateway/cache", nil)
	t.Logf("  %s", c)
	ws12AssertSane(t, c)
	if c.Status < 500 {
		t.Errorf("DELETE /gateway/cache with Redis down: %d, want 5xx", c.Status)
	}
	if strings.Contains(c.Body, "dial tcp") {
		t.Logf("note: cache invalidation error leaks internal dial address: %s", c.Body)
	}
	// FINDING WS12-3: each gateway call pays ~5s of go-redis dial retries.
	if g := down["gateway"]; g.Err == nil && g.Elapsed > 2*time.Second {
		t.Errorf("FINDING WS12-3: gateway call with Redis down took %s (healthy: %s) — rate-limit/cache Redis calls retry for seconds instead of failing fast",
			g.Elapsed.Round(time.Millisecond), before["gateway"].Elapsed.Round(time.Millisecond))
	}
	if log := logSince(); !strings.Contains(log, "rate limit check failed") {
		t.Errorf("no 'rate limit check failed' in server.log; fail-open is invisible")
	}

	ws12Compose(t, "start", "redis")
	t.Logf("redis healthy %s after start", ws12WaitHealthy(t, "redis", 60*time.Second).Round(time.Millisecond))
	ws12AssertRecovered(t)
	// Rate limiting is enforced again (headers come back).
	r := Bearer(FX(t).URLs.Gateway, FX(t).Personas["gw_key"].Secret).Post(t, "/chat/completions", Chat("e2e-ok", Uniq("ws12 rl"), false))
	if r.Header.Get("X-RateLimit-Limit") == "" {
		t.Errorf("no X-RateLimit-Limit after Redis recovered: %v", r.Header)
	}
	ws12AssertNoPanic(t, logSince())
}

// IAMKit restarted while traffic flows: every response is 200 or a quick JSON
// 401/5xx, nothing hangs or resets, and FreeRouter recovers on its own —
// including the backend service account (iamx machine token) path.
func TestWS12_Outage_IAMKitRestartMidRun(t *testing.T) {
	logSince := ws12ServerLogMark(t)
	if p := ws12Call("access", AdminAPI(t), "GET", "/access/users?limit=1", nil); p.Status != 200 {
		t.Fatalf("precondition %s", p)
	}
	t.Cleanup(func() {
		if ws12Health("iamkit") != "healthy" {
			ws12Compose(t, "start", "iamkit")
			ws12WaitHealthy(t, "iamkit", 120*time.Second)
		}
	})

	f := FX(t)
	gw := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret)
	ctx, stop := context.WithCancel(context.Background())
	var mu sync.Mutex
	var probes []ws12Probe
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for ctx.Err() == nil {
				var p ws12Probe
				if w == 0 {
					p = ws12Call("gateway", gw, "POST", "/chat/completions", Chat("e2e-ok", Uniq("ws12 mid"), false))
				} else {
					p = ws12Call("access", AdminAPI(t), "GET", "/access/users?limit=1", nil)
				}
				mu.Lock()
				probes = append(probes, p)
				mu.Unlock()
				// gw_key has a 60 RPM default limit shared with the rest of
				// this file: stay well below it.
				time.Sleep(time.Duration(1+2*(1-w)) * 400 * time.Millisecond)
			}
		}(w)
	}
	time.Sleep(time.Second)
	restartStart := time.Now()
	ws12Compose(t, "restart", "iamkit")
	t.Logf("docker compose restart iamkit took %s; healthy %s later", time.Since(restartStart).Round(time.Millisecond),
		ws12WaitHealthy(t, "iamkit", 120*time.Second).Round(time.Millisecond))
	time.Sleep(2 * time.Second)
	stop()
	wg.Wait()

	counts := map[string]map[int]int{}
	var worst time.Duration
	for _, p := range probes {
		ws12AssertSane(t, p)
		if counts[p.Name] == nil {
			counts[p.Name] = map[int]int{}
		}
		counts[p.Name][p.Status]++
		if p.Elapsed > worst {
			worst = p.Elapsed
		}
		if p.Err == nil && p.Status != 200 && p.Status != 401 && p.Status < 500 {
			t.Errorf("unexpected status during restart: %s", p)
		}
	}
	t.Logf("during restart: %d calls, statuses %v, slowest %s", len(probes), counts, worst.Round(time.Millisecond))
	ws12AssertRecovered(t)
	ws12AssertNoPanic(t, logSince())
}
