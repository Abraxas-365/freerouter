//go:build e2e

package api

// WS-07 — resilience (fallback, retry, timeouts, aborts), key health,
// routing strategies and the response cache.

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ws07LocalStatus returns a local upstream that always answers with status.
func ws07LocalStatus(t *testing.T, status int) *ws07Local {
	return ws07NewLocal(t, func(w http.ResponseWriter, _ *http.Request, _ *ws07LocalReq) {
		if status == 200 {
			ws07WriteChatOK(w, "local ok")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":{"message":"local failure"}}`))
	})
}

// ── Fallback & retry ─────────────────────────────────────────────────

func TestWS07_Fallback_500ToFallbackModel(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-fb")
	r := gw.Post(t, "/chat/completions", Chat("e2e-fallback", marker, false))
	if r.Status != 200 {
		t.Fatalf("want 200 via fallback, got %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["model"] != "e2e-fallback" || ws07ChatContent(t, m) != "Hello from fakellm!" {
		t.Errorf("body %s", r.Body)
	}
	recs := ws07UpstreamWith(t, marker)
	if len(recs) != 2 || recs[0].Model != "fake-500" || recs[1].Model != "fake-ok" {
		t.Fatalf("upstream sequence %v", recs)
	}
	if gap := recs[1].At.Sub(recs[0].At); gap < 450*time.Millisecond {
		t.Errorf("no backoff between attempts (%s)", gap)
	}
	// Usage: one failed primary row, one successful fallback row.
	Eventually(t, 10*time.Second, func() bool {
		var fail, ok bool
		for _, row := range ws07Usage(t, "e2e-fallback") {
			if row["used_model"] == "fake-500" && row["has_error"] == true && ws07Num(row["status_code"]) == 500 {
				fail = true
			}
			if row["used_model"] == "fake-ok" && row["is_fallback"] == true && ws07Num(row["total_tokens"]) == 9 {
				ok = true
			}
		}
		return fail && ok
	}, "failed-primary and is_fallback usage rows")
}

func TestWS07_Retry_429ToNextRoute(t *testing.T) {
	gw := ws07NewGateway(t)
	seg := Uniq("r429")
	p1 := ws07NewProvider(t, "openai", seg+"-a")
	p2 := ws07NewProvider(t, "openai", seg+"-b")
	mid, name := ws07NewModel(t, "r429")
	ws07Map(t, mid, p1.ID, "fake-429", 1, 1, nil) // cheapest → first
	ws07Map(t, mid, p2.ID, "fake-ok", 1, 9, nil)
	r := gw.Post(t, "/chat/completions", Chat(name, "x", false))
	if r.Status != 200 {
		t.Fatalf("429 then ok: want 200, got %d %s", r.Status, r.Body)
	}
	if a, b := len(ws07Upstream(t, p1.Seg)), len(ws07Upstream(t, p2.Seg)); a != 1 || b != 1 {
		t.Errorf("upstream calls: 429-route=%d ok-route=%d", a, b)
	}
}

func TestWS07_NonRetryable400_NoFallback(t *testing.T) {
	gw := ws07NewGateway(t)
	seg := Uniq("r400")
	p1 := ws07NewProvider(t, "openai", seg+"-a")
	p2 := ws07NewProvider(t, "openai", seg+"-b")
	mid, name := ws07NewModel(t, "r400")
	ws07Map(t, mid, p1.ID, "fake-400", 1, 1, nil)
	ws07Map(t, mid, p2.ID, "fake-ok", 1, 9, nil)
	r := gw.Post(t, "/chat/completions", Chat(name, "x", false))
	if r.Status < 400 || r.Status == 200 {
		t.Fatalf("upstream 400 should be surfaced, got %d %s", r.Status, r.Body)
	}
	if !strings.Contains(string(r.Body), "simulated bad request") {
		t.Errorf("client error not surfaced: %s", r.Body)
	}
	if n := len(ws07Upstream(t, p2.Seg)); n != 0 {
		t.Errorf("a client error (400) was retried on another route %d times", n)
	}
}

func TestWS07_AllRoutesFail(t *testing.T) {
	gw := ws07NewGateway(t)
	seg := Uniq("allfail")
	p1 := ws07NewProvider(t, "openai", seg+"-a")
	p2 := ws07NewProvider(t, "openai", seg+"-b")
	mid, name := ws07NewModel(t, "allfail")
	ws07Map(t, mid, p1.ID, "fake-500", 1, 1, nil)
	ws07Map(t, mid, p2.ID, "fake-500", 1, 2, nil)
	r := gw.Post(t, "/chat/completions", Chat(name, "x", false))
	if r.Status != http.StatusBadGateway {
		t.Fatalf("want 502, got %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["code"] != "EXTERNAL" || !strings.Contains(m["message"].(string), "upstream returned 500") {
		t.Errorf("error body %s", r.Body)
	}
	if a, b := len(ws07Upstream(t, p1.Seg)), len(ws07Upstream(t, p2.Seg)); a != 1 || b != 1 {
		t.Errorf("each route should be tried once: %d / %d", a, b)
	}
	t.Logf("all-routes-fail latency: %s", r.Elapsed)
	if r.Elapsed > 5*time.Second {
		t.Errorf("all-fail took %s", r.Elapsed)
	}
}

// FINDING WS07-16: upstream 429 (all routes rate-limited) becomes 502 and the
// client loses the rate-limit signal (status and Retry-After).
func TestWS07_Upstream429_SurfacedAs429(t *testing.T) {
	gw := ws07NewGateway(t)
	r := gw.Post(t, "/chat/completions", Chat("e2e-429", Uniq("ws07-429"), false))
	if r.Status != http.StatusTooManyRequests {
		t.Errorf("all routes 429: want 429 (+Retry-After), got %d %s", r.Status, r.Body)
	}
}

// FINDING WS07-8: a transport error (connection refused/DNS/reset) is not
// retried on the next route — the request fails with 500.
func TestWS07_ConnectionRefused_FallsBack(t *testing.T) {
	gw := ws07NewGateway(t)
	seg := Uniq("dead")
	dead := ws07NewProvider(t, "openai", seg+"-dead", "http://127.0.0.1:1/v1")
	ok := ws07NewProvider(t, "openai", seg+"-ok")
	mid, name := ws07NewModel(t, "dead")
	ws07Map(t, mid, dead.ID, "fake-ok", 1, 1, nil) // cheapest → tried first
	ws07Map(t, mid, ok.ID, "fake-ok", 1, 5, nil)
	r := gw.Post(t, "/chat/completions", Chat(name, "x", false))
	if r.Status != 200 {
		t.Errorf("first route unreachable, second healthy: want 200, got %d %s (healthy route calls: %d)",
			r.Status, r.Body, len(ws07Upstream(t, ok.Seg)))
	}
}

// FINDING WS07-15: a 200 upstream with an unparsable body returns 500 INTERNAL
// (not 502) and is not retried on the next route.
func TestWS07_MalformedUpstreamJSON(t *testing.T) {
	gw := ws07NewGateway(t)
	r := gw.Post(t, "/chat/completions", Chat("e2e-malformed", Uniq("ws07-malf"), false))
	if r.Status != http.StatusBadGateway {
		t.Errorf("malformed upstream JSON: want 502, got %d %s", r.Status, r.Body)
	}
	if strings.Contains(string(r.Body), "chatcmpl-broken") {
		t.Errorf("raw upstream body leaked: %s", r.Body)
	}
	Eventually(t, 10*time.Second, func() bool {
		for _, row := range ws07Usage(t, "e2e-malformed") {
			if row["has_error"] == true {
				return true
			}
		}
		return false
	}, "malformed response logged as error")
}

func TestWS07_NoUsageUpstream(t *testing.T) {
	gw := ws07NewGateway(t)
	r := gw.Post(t, "/chat/completions", Chat("e2e-nousage", Uniq("ws07-nou"), false))
	if r.Status != 200 || ws07ChatContent(t, r.Map(t)) != "Hello from fakellm!" {
		t.Fatalf("%d %s", r.Status, r.Body)
	}
	rows := ws07WaitUsage(t, "e2e-nousage", 1)
	if rows[0]["has_error"] != false || ws07Num(rows[0]["total_tokens"]) != 0 || ws07Num(rows[0]["total_cost"]) != 0 {
		t.Errorf("row %v", rows[0])
	}
}

// ── Timeouts & client disconnect ─────────────────────────────────────

func TestWS07_SlowUpstream_Measured(t *testing.T) {
	gw := ws07NewGateway(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	r, err := gw.DoCtx(ctx, http.MethodPost, "/chat/completions", Chat("e2e-slow", Uniq("ws07-slow"), false))
	if err != nil {
		t.Fatalf("slow call: %v", err)
	}
	t.Logf("e2e-slow: status=%d elapsed=%s", r.Status, r.Elapsed)
	if r.Status != 200 || r.Elapsed < 15*time.Second {
		t.Errorf("e2e-slow (upstream sleeps 20s): status %d after %s", r.Status, r.Elapsed)
	}
}

// FINDING WS07-20: the upstream timeout is a hard-coded 5 minutes; a hung
// primary blocks the request (and its fallback) for minutes.
func TestWS07_HungUpstream_BoundedTimeout(t *testing.T) {
	gw := ws07NewGateway(t)
	hang := ws07NewLocal(t, func(w http.ResponseWriter, r *http.Request, _ *ws07LocalReq) {
		select {
		case <-r.Context().Done():
		case <-time.After(60 * time.Second):
		}
	})
	seg := Uniq("hang")
	ph := ws07NewProvider(t, "openai", seg+"-h", hang.URL+"/v1")
	pok := ws07NewProvider(t, "openai", seg+"-ok")
	mid, name := ws07NewModel(t, "hang")
	ws07Map(t, mid, ph.ID, "x", 1, 1, nil)
	ws07Map(t, mid, pok.ID, "fake-ok", 1, 5, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	start := time.Now()
	r, err := gw.DoCtx(ctx, http.MethodPost, "/chat/completions", Chat(name, "x", false))
	el := time.Since(start)
	t.Logf("hung primary: err=%v status=%d elapsed=%s", err, r.Status, el)
	if err != nil || el > 35*time.Second {
		t.Errorf("hung upstream not bounded: gateway had not answered after %s (fallback route available)", el)
	}
}

// FINDING WS07-9: a client disconnect does not cancel the upstream call; the
// per-subject concurrency slot stays held until the upstream finishes.
func TestWS07_ClientDisconnect_CancelsUpstream(t *testing.T) {
	gw := ws07NewGateway(t)
	ws07WithConcurrency(t, gw, 1)
	var cancelled atomic.Int64
	hang := ws07NewLocal(t, func(w http.ResponseWriter, r *http.Request, _ *ws07LocalReq) {
		start := time.Now()
		select {
		case <-r.Context().Done():
			cancelled.Store(int64(time.Since(start)))
		case <-time.After(25 * time.Second):
			ws07WriteChatOK(w, "late")
		}
	})
	p := ws07NewProvider(t, "openai", Uniq("disc"), hang.URL+"/v1")
	mid, name := ws07NewModel(t, "disc")
	ws07Map(t, mid, p.ID, "x", 1, 1, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	_, err := gw.DoCtx(ctx, http.MethodPost, "/chat/completions", Chat(name, "x", false))
	cancel()
	if err == nil {
		t.Fatal("expected client-side timeout")
	}
	time.Sleep(1500 * time.Millisecond)
	r := gw.Post(t, "/chat/completions", Chat("e2e-ok", Uniq("ws07-after-disc"), false))
	if r.Status != 200 {
		t.Errorf("3s after the client went away (max_concurrent=1) a new request got %d %s — slot still held", r.Status, r.Body)
	}
	if cancelled.Load() == 0 {
		t.Errorf("upstream request was not cancelled after the client disconnected")
	}
}

// ── Streaming failure modes ──────────────────────────────────────────

func TestWS07_StreamAbort_ClientAndUsage(t *testing.T) {
	gw := ws07NewGateway(t)
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-stream-abort", Uniq("ws07-abort"), true), 20*time.Second)
	t.Logf("client saw: status=%d events=%d readErr=%v done=%v raw=%q", s.Status, len(s.Events), s.ReadErr, s.HasDone(), s.Raw)
	if s.HasDone() {
		t.Errorf("aborted upstream stream ended with [DONE] — indistinguishable from success")
	}
	if got := ws07StreamText(t, s.DataJSON(t)); got != "Hello " {
		t.Errorf("partial text %q", got)
	}
	Eventually(t, 10*time.Second, func() bool {
		for _, row := range ws07Usage(t, "e2e-stream-abort") {
			if row["streamed"] == true && row["has_error"] == true && strings.Contains(row["error_message"].(string), "EOF") {
				return true
			}
		}
		return false
	}, "stream abort logged with has_error=true")
}

// FINDING WS07-14: a mid-stream upstream abort gives the client a clean,
// normally-terminated response with no error event (contrast /v1/messages and /v1/responses).
func TestWS07_StreamAbort_ClientGetsErrorSignal(t *testing.T) {
	gw := ws07NewGateway(t)
	s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-stream-abort", Uniq("ws07-abort2"), true), 20*time.Second)
	sawErr := s.ReadErr != nil
	for _, e := range s.Events {
		if strings.Contains(e.Data, `"error"`) {
			sawErr = true
		}
	}
	if !sawErr {
		t.Errorf("upstream aborted mid-stream but the client saw a clean EOF with no error event/connection reset:\n%s", s.Raw)
	}
}

// FINDING WS07-10: streaming requests whose upstream fails before the first
// byte return HTTP 200 with an empty body, and never try the next route/fallback.
func TestWS07_StreamUpstreamError(t *testing.T) {
	gw := ws07NewGateway(t)
	t.Run("all routes fail", func(t *testing.T) {
		s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-500", Uniq("ws07-s500"), true), 20*time.Second)
		if s.Status == 200 && !strings.Contains(s.Raw, "error") {
			t.Errorf("stream to failing upstream: HTTP 200 with body %q (Content-Length %q) — no error status or event",
				s.Raw, s.Header.Get("Content-Length"))
		}
	})
	t.Run("fallback model", func(t *testing.T) {
		marker := Uniq("ws07-sfb")
		s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-fallback", marker, true), 20*time.Second)
		if got := ws07StreamText(t, s.DataJSON(t)); got != "Hello from fakellm! " {
			t.Errorf("stream with fallback: status %d text %q; upstream calls %d (fallback never tried)",
				s.Status, got, len(ws07UpstreamWith(t, marker)))
		}
	})
}

// ── Key health ───────────────────────────────────────────────────────

func TestWS07_KeyHealth_DegradedThenSwitchAndWebhook(t *testing.T) {
	gw := ws07NewGateway(t)
	hook := ws07Webhook(t, Uniq("ws07-health"), "key.health_degraded", "key.blacklisted")
	up := ws07NewLocal(t, func(w http.ResponseWriter, r *http.Request, _ *ws07LocalReq) {
		if r.Header.Get("Authorization") == "Bearer bad-key" {
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"error":{"message":"overloaded"}}`))
			return
		}
		ws07WriteChatOK(w, "good key")
	})
	p := ws07NewProvider(t, "openai", Uniq("health"), up.URL+"/v1")
	// Only a failing key at first: deactivate the default key, add bad-key.
	admin := AdminAPI(t)
	if r := admin.Put(t, "/provider-keys/"+p.KeyID, map[string]any{"status": "inactive"}); r.Status != 200 && r.Status != 204 {
		t.Fatalf("deactivate default key: %d %s", r.Status, r.Body)
	}
	bad := ws07AddKey(t, p.ID, "ws07-bad", "bad-key", "")
	mid, name := ws07NewModel(t, "health")
	ws07Map(t, mid, p.ID, "x", 1, 1, nil)

	for i := 1; i <= 3; i++ {
		r := gw.Post(t, "/chat/completions", Chat(name, Uniq("x"), false))
		if r.Status != http.StatusBadGateway {
			t.Fatalf("call %d with failing key: want 502, got %d %s", i, r.Status, r.Body)
		}
	}
	// Add a healthy key: the degraded key must be skipped from now on.
	ws07AddKey(t, p.ID, "ws07-good", "good-key", "")
	for i := 0; i < 3; i++ {
		r := gw.Post(t, "/chat/completions", Chat(name, Uniq("x"), false))
		if r.Status != 200 || ws07ChatContent(t, r.Map(t)) != "good key" {
			t.Fatalf("after 3 consecutive errors the degraded key should be skipped: %d %s", r.Status, r.Body)
		}
	}
	reqs := up.Reqs()
	if len(reqs) != 6 {
		t.Errorf("upstream calls %d (want 3 bad + 3 good, no retries on the degraded key)", len(reqs))
	}
	Eventually(t, 15*time.Second, func() bool {
		for _, b := range ws07SinkBodies(t, hook) {
			d, _ := b["data"].(map[string]any)
			if b["event"] == "key.health_degraded" && d["key_id"] == bad && ws07Num(d["status_code"]) == 503 {
				return true
			}
		}
		return false
	}, "key.health_degraded webhook for the failing key")
}

func TestWS07_KeyHealth_AuthErrorBlacklists(t *testing.T) {
	gw := ws07NewGateway(t)
	hook := ws07Webhook(t, Uniq("ws07-bl"), "key.blacklisted")
	up := ws07NewLocal(t, func(w http.ResponseWriter, r *http.Request, _ *ws07LocalReq) {
		if r.Header.Get("Authorization") == "Bearer revoked" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
			return
		}
		ws07WriteChatOK(w, "valid key")
	})
	p := ws07NewProvider(t, "openai", Uniq("bl"), up.URL+"/v1")
	admin := AdminAPI(t)
	admin.Put(t, "/provider-keys/"+p.KeyID, map[string]any{"status": "inactive"})
	revoked := ws07AddKey(t, p.ID, "ws07-revoked", "revoked", "")
	time.Sleep(20 * time.Millisecond)
	ws07AddKey(t, p.ID, "ws07-valid", "valid", "")
	mid, name := ws07NewModel(t, "bl")
	ws07Map(t, mid, p.ID, "x", 1, 1, nil)

	r1 := gw.Post(t, "/chat/completions", Chat(name, "x", false))
	t.Logf("first call (revoked key): %d %s", r1.Status, r1.Body)
	r2 := gw.Post(t, "/chat/completions", Chat(name, "x", false))
	if r2.Status != 200 || ws07ChatContent(t, r2.Map(t)) != "valid key" {
		t.Fatalf("after 401 the key must be blacklisted and the next key used: %d %s", r2.Status, r2.Body)
	}
	Eventually(t, 15*time.Second, func() bool {
		for _, b := range ws07SinkBodies(t, hook) {
			if d, _ := b["data"].(map[string]any); b["event"] == "key.blacklisted" && d["key_id"] == revoked {
				return true
			}
		}
		return false
	}, "key.blacklisted webhook")
}

// FINDING WS07-17: route ordering ignores key health. A cheaper route whose
// only key is degraded is still tried first on every request (+500ms backoff).
func TestWS07_KeyHealth_DegradedRouteDeprioritized(t *testing.T) {
	gw := ws07NewGateway(t)
	fail := ws07LocalStatus(t, 503)
	seg := Uniq("deprio")
	pf := ws07NewProvider(t, "openai", seg+"-f", fail.URL+"/v1")
	pok := ws07NewProvider(t, "openai", seg+"-ok")
	mid, name := ws07NewModel(t, "deprio")
	ws07Map(t, mid, pf.ID, "x", 1, 1, nil)
	ws07Map(t, mid, pok.ID, "fake-ok", 1, 5, nil)
	var slow int
	for i := 0; i < 6; i++ {
		r := gw.Post(t, "/chat/completions", Chat(name, Uniq("x"), false)) // unique: avoid cache hits
		if r.Status != 200 {
			t.Fatalf("call %d: %d %s", i, r.Status, r.Body)
		}
		if r.Elapsed > 450*time.Millisecond {
			slow++
		}
	}
	if n := fail.Count(); n > 3 {
		t.Errorf("degraded route tried on %d/6 requests (%d paid the retry backoff); expected it to be skipped after 3 consecutive errors", n, slow)
	}
}

// ── Routing strategies ───────────────────────────────────────────────

func TestWS07_Strategy_Cheapest(t *testing.T) {
	gw := ws07NewGateway(t)
	ws07SetStrategy(t, gw.Subject, "cheapest")
	seg := Uniq("cheap")
	exp := ws07NewProvider(t, "openai", seg+"-exp")
	chp := ws07NewProvider(t, "openai", seg+"-chp")
	mid, name := ws07NewModel(t, "cheap")
	ws07Map(t, mid, exp.ID, "fake-ok", 1, 9, nil) // created first, more expensive
	ws07Map(t, mid, chp.ID, "fake-ok", 1, 1, nil)
	for i := 0; i < 10; i++ {
		if r := gw.Post(t, "/chat/completions", Chat(name, Uniq("x"), false)); r.Status != 200 {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
	}
	if a, b := len(ws07Upstream(t, exp.Seg)), len(ws07Upstream(t, chp.Seg)); a != 0 || b != 10 {
		t.Errorf("cheapest: expensive=%d cheap=%d (want 0/10)", a, b)
	}
	rows := ws07WaitUsage(t, name, 10)
	for _, row := range rows {
		if row["provider_id"] != chp.ID {
			t.Errorf("usage row billed to provider %v", row["provider_id"])
			break
		}
	}
}

func TestWS07_Strategy_RoundRobinDistribution(t *testing.T) {
	gw := ws07NewGateway(t)
	ws07SetStrategy(t, gw.Subject, "round-robin")
	seg := Uniq("rr")
	var provs []ws07Prov
	mid, name := ws07NewModel(t, "rr")
	for i, s := range []string{"a", "b", "c"} {
		p := ws07NewProvider(t, "openai", seg+"-"+s)
		ws07Map(t, mid, p.ID, "fake-ok", 1, float64(i+1), nil)
		provs = append(provs, p)
	}
	const n = 60
	for i := 0; i < n; i++ {
		if r := gw.Post(t, "/chat/completions", Chat(name, Uniq("x"), false)); r.Status != 200 {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
	}
	total := 0
	for _, p := range provs {
		c := len(ws07Upstream(t, p.Seg))
		total += c
		t.Logf("%s: %d/%d", p.Seg, c, n)
		if c < 8 {
			t.Errorf("provider %s got only %d/%d requests", p.Seg, c, n)
		}
	}
	if total != n {
		t.Errorf("total upstream calls %d (want %d — no retries expected)", total, n)
	}
}

// FINDING WS07-19: "round-robin" is a random shuffle, not a rotation.
func TestWS07_Strategy_RoundRobinRotates(t *testing.T) {
	gw := ws07NewGateway(t)
	ws07SetStrategy(t, gw.Subject, "round-robin")
	seg := Uniq("rot")
	mid, name := ws07NewModel(t, "rot")
	pa := ws07NewProvider(t, "openai", seg+"-a")
	pb := ws07NewProvider(t, "openai", seg+"-b")
	ws07Map(t, mid, pa.ID, "fake-ok", 1, 1, nil)
	ws07Map(t, mid, pb.ID, "fake-ok", 1, 2, nil)
	var seq []string
	for i := 0; i < 12; i++ {
		marker := Uniq("ws07-rot")
		gw.Post(t, "/chat/completions", Chat(name, marker, false))
		for _, r := range ws07UpstreamWith(t, marker) {
			seq = append(seq, strings.Split(r.Path, "/")[2])
		}
	}
	for i := 1; i < len(seq); i++ {
		if seq[i] == seq[i-1] {
			t.Errorf("same route twice in a row with 2 routes under round-robin: %v", seq)
			break
		}
	}
}

// FINDING WS07-18: lowest-latency never measures routes it has not used, so it
// sticks to the first-created route even when another is much faster.
func TestWS07_Strategy_LowestLatency(t *testing.T) {
	gw := ws07NewGateway(t)
	ws07SetStrategy(t, gw.Subject, "lowest-latency")
	slow := ws07NewLocal(t, func(w http.ResponseWriter, _ *http.Request, _ *ws07LocalReq) {
		time.Sleep(400 * time.Millisecond)
		ws07WriteChatOK(w, "slow")
	})
	fast := ws07NewLocal(t, func(w http.ResponseWriter, _ *http.Request, _ *ws07LocalReq) { ws07WriteChatOK(w, "fast") })
	seg := Uniq("lat")
	ps := ws07NewProvider(t, "openai", seg+"-s", slow.URL+"/v1")
	pf := ws07NewProvider(t, "openai", seg+"-f", fast.URL+"/v1")
	mid, name := ws07NewModel(t, "lat")
	ws07Map(t, mid, ps.ID, "x", 1, 1, nil)
	ws07Map(t, mid, pf.ID, "x", 1, 1, nil)
	for i := 0; i < 10; i++ {
		if r := gw.Post(t, "/chat/completions", Chat(name, Uniq("x"), false)); r.Status != 200 {
			t.Fatalf("%d %s", r.Status, r.Body)
		}
	}
	t.Logf("slow(400ms)=%d fast=%d", slow.Count(), fast.Count())
	if fast.Count() < 7 {
		t.Errorf("lowest-latency: fast route got %d/10 (slow route %d/10)", fast.Count(), slow.Count())
	}
}

func TestWS07_Strategy_ConfigValidation(t *testing.T) {
	admin := AdminAPI(t)
	gw := ws07NewGateway(t)
	if r := admin.Post(t, "/routing-configs", map[string]any{"subject_id": gw.Subject, "strategy": "fastest-please"}); r.Status != 400 {
		t.Errorf("invalid strategy: %d %s", r.Status, r.Body)
	}
	ws07SetStrategy(t, gw.Subject, "cheapest")
	if r := admin.Post(t, "/routing-configs", map[string]any{"subject_id": gw.Subject, "strategy": "round-robin"}); r.Status != http.StatusConflict {
		t.Errorf("duplicate subject routing config: %d %s", r.Status, r.Body)
	}
}

// ── Response cache ───────────────────────────────────────────────────

func TestWS07_Cache_HitMissAndInvalidate(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-cache")
	body := Chat("e2e-ok", marker, false)
	r1 := gw.Post(t, "/chat/completions", body)
	r2 := gw.Post(t, "/chat/completions", body)
	if r1.Status != 200 || r2.Status != 200 {
		t.Fatalf("%d / %d", r1.Status, r2.Status)
	}
	if r1.Header.Get("X-Cache") != "MISS" || r2.Header.Get("X-Cache") != "HIT" {
		t.Errorf("X-Cache %q then %q", r1.Header.Get("X-Cache"), r2.Header.Get("X-Cache"))
	}
	if ws07ChatContent(t, r2.Map(t)) != "Hello from fakellm!" || r2.Map(t)["model"] != "e2e-ok" {
		t.Errorf("cached body %s", r2.Body)
	}
	if n := len(ws07UpstreamWith(t, marker)); n != 1 {
		t.Errorf("upstream calls %d (want 1)", n)
	}
	// Different params → MISS.
	alt := Chat("e2e-ok", marker, false)
	alt["temperature"] = 0.9
	if r := gw.Post(t, "/chat/completions", alt); r.Header.Get("X-Cache") != "MISS" {
		t.Errorf("different temperature should miss: %q", r.Header.Get("X-Cache"))
	}
	// Different subject → MISS (cache is per subject).
	other := ws07OtherGateway(t)
	if r := other.Post(t, "/chat/completions", body); r.Header.Get("X-Cache") != "MISS" {
		t.Errorf("other subject got %q (cross-tenant cache leak)", r.Header.Get("X-Cache"))
	}
	// Subject-scoped invalidation.
	inv := Bearer(FX(t).URLs.API, FX(t).Personas["admin_key"].Secret).Delete(t, "/gateway/cache?subject="+gw.Subject)
	if inv.Status != 200 || ws07Num(inv.Map(t)["invalidated"]) < 1 {
		t.Fatalf("invalidate: %d %s", inv.Status, inv.Body)
	}
	if r := gw.Post(t, "/chat/completions", body); r.Header.Get("X-Cache") != "MISS" {
		t.Errorf("after invalidation: %q", r.Header.Get("X-Cache"))
	}
	// Other subject's entry untouched.
	if r := other.Post(t, "/chat/completions", body); r.Header.Get("X-Cache") != "HIT" {
		t.Errorf("subject-scoped invalidation cleared another subject: %q", r.Header.Get("X-Cache"))
	}
	// Invalidation requires gateway:write.
	if r := Bearer(FX(t).URLs.API, gw.Secret).Delete(t, "/gateway/cache"); r.Status != 403 {
		t.Errorf("gateway:invoke-only key invalidating cache: %d", r.Status)
	}
}

func TestWS07_Cache_TTLExpiry(t *testing.T) {
	gw := ws07NewGateway(t)
	body := Chat("e2e-ok", Uniq("ws07-ttl"), false)
	gw.Post(t, "/chat/completions", body)
	if r := gw.Post(t, "/chat/completions", body); r.Header.Get("X-Cache") != "HIT" {
		t.Fatalf("second call %q", r.Header.Get("X-Cache"))
	}
	time.Sleep(6500 * time.Millisecond) // CACHE_TTL_SECONDS=5 in the e2e env
	if r := gw.Post(t, "/chat/completions", body); r.Header.Get("X-Cache") != "MISS" {
		t.Errorf("after TTL: %q", r.Header.Get("X-Cache"))
	}
}

func TestWS07_Cache_StreamingNotCached(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-cstream")
	for i := 0; i < 2; i++ {
		s := ws07Stream(t, gw.Client, "/chat/completions", Chat("e2e-ok", marker, true), 15*time.Second)
		if s.Status != 200 || s.Header.Get("X-Cache") != "" {
			t.Errorf("stream %d: status %d X-Cache %q", i, s.Status, s.Header.Get("X-Cache"))
		}
	}
	if n := len(ws07UpstreamWith(t, marker)); n != 2 {
		t.Errorf("streamed twice → %d upstream calls (want 2)", n)
	}
	// A non-stream call with identical content is a MISS (stream never populated the cache).
	if r := gw.Post(t, "/chat/completions", Chat("e2e-ok", marker, false)); r.Header.Get("X-Cache") != "MISS" {
		t.Errorf("non-stream after streams: %q", r.Header.Get("X-Cache"))
	}
}

func TestWS07_Cache_ErrorsNotCached(t *testing.T) {
	gw := ws07NewGateway(t)
	marker := Uniq("ws07-cerr")
	for i := 0; i < 2; i++ {
		r := gw.Post(t, "/chat/completions", Chat("e2e-500", marker, false))
		if r.Status != 502 || r.Header.Get("X-Cache") == "HIT" {
			t.Errorf("call %d: %d X-Cache %q", i, r.Status, r.Header.Get("X-Cache"))
		}
	}
	if n := len(ws07UpstreamWith(t, marker)); n != 2 {
		t.Errorf("upstream calls %d (want 2)", n)
	}
}

func TestWS07_Cache_MessagesAndResponses(t *testing.T) {
	gw := ws07NewGateway(t)
	mm := Uniq("ws07-cmsg")
	mb := map[string]any{"model": "e2e-ok", "max_tokens": 10, "messages": []any{map[string]any{"role": "user", "content": mm}}}
	gw.Post(t, "/messages", mb)
	r := gw.Post(t, "/messages", mb)
	if r.Header.Get("X-Cache") != "HIT" || r.Map(t)["type"] != "message" {
		t.Errorf("/messages second call X-Cache=%q body=%s", r.Header.Get("X-Cache"), r.Body)
	}
	rm := Uniq("ws07-cresp")
	rb := map[string]any{"model": "e2e-ok", "input": rm}
	gw.Post(t, "/responses", rb)
	r = gw.Post(t, "/responses", rb)
	if r.Header.Get("X-Cache") != "HIT" || r.Map(t)["object"] != "response" {
		t.Errorf("/responses second call X-Cache=%q body=%s", r.Header.Get("X-Cache"), r.Body)
	}
	if n := len(ws07UpstreamWith(t, mm)) + len(ws07UpstreamWith(t, rm)); n != 2 {
		t.Errorf("upstream calls %d (want 2)", n)
	}
}

// TestWS07_ZZZ_Cleanup revokes the shared WS-07 service accounts (runs last: tests run in source order, this file sorts after endpoints).
func TestWS07_ZZZ_Cleanup(t *testing.T) { ws07CleanupPool(t) }
