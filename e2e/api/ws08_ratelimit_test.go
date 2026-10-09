//go:build e2e

package api

// WS-08 rate limits: per-subject RPM + concurrency enforcement on the
// gateway, config CRUD/validation, Redis key hygiene. Only the WS-08
// service account's own limit row (and ws08-probe-* subjects) are touched;
// the global "default" config is never created or modified.

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ws08Chat sends one uncached chat request through the WS-08 subject.
func ws08Chat(t *testing.T, gw ws08GW, model string) Resp {
	t.Helper()
	return gw.Post(t, "/chat/completions", Chat(model, Uniq("ws08-rl-prompt"), false))
}

func ws08AssertRateLimited(t *testing.T, r Resp, wantLimit string) {
	t.Helper()
	if r.Status != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d %s", r.Status, r.Body)
	}
	m := r.Map(t)
	if m["code"] != "RATE_LIMITED" || m["message"] != "rate limit exceeded" {
		t.Errorf("429 body = %s, want code RATE_LIMITED / message \"rate limit exceeded\"", r.Body)
	}
	if got := r.Header.Get("X-RateLimit-Limit"); got != wantLimit {
		t.Errorf("X-RateLimit-Limit = %q, want %q", got, wantLimit)
	}
	if got := r.Header.Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("X-RateLimit-Remaining = %q, want 0", got)
	}
	ra, err := strconv.Atoi(r.Header.Get("Retry-After"))
	if err != nil || ra < 1 {
		t.Errorf("Retry-After = %q, want integer seconds >= 1", r.Header.Get("Retry-After"))
	}
}

func TestWS08_RateLimit_RPMExceeded(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("rpm"), "fake-ok")
	ws08SetLimit(t, gw, 3, 0)

	for i := 1; i <= 3; i++ {
		r := ws08Chat(t, gw, model)
		if r.Status != 200 {
			t.Fatalf("request %d within limit: %d %s", i, r.Status, r.Body)
		}
		if got := r.Header.Get("X-RateLimit-Limit"); got != "3" {
			t.Errorf("request %d X-RateLimit-Limit = %q, want 3", i, got)
		}
		if r.Header.Get("Retry-After") != "" {
			t.Errorf("request %d: Retry-After on a 200", i)
		}
	}
	r := ws08Chat(t, gw, model)
	ws08AssertRateLimited(t, r, "3")

	// Rejected calls must not consume window slots, and the window key expires.
	rdb := ws08Redis(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ws08Chat(t, gw, model)
	}
	if n := rdb.ZCard(ctx, "ratelimit:rpm:"+gw.Subject).Val(); n != 3 {
		t.Errorf("ratelimit:rpm zset has %d members after 3 admitted + 4 rejected, want 3", n)
	}
	if ttl := rdb.TTL(ctx, "ratelimit:rpm:"+gw.Subject).Val(); ttl <= 0 || ttl > time.Minute {
		t.Errorf("ratelimit:rpm TTL = %s, want (0, 1m]", ttl)
	}
	if ex := rdb.Exists(ctx, "ratelimit:concurrent:"+gw.Subject).Val(); ex != 0 {
		t.Errorf("concurrency key exists although max_concurrent=0")
	}
}

// FINDING WS08-1: X-RateLimit-Remaining is always rpm-1 on admitted requests.
func TestWS08_RateLimit_RemainingDecrements(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("rem"), "fake-ok")
	ws08SetLimit(t, gw, 4, 0)
	var got []string
	for i := 0; i < 4; i++ {
		r := ws08Chat(t, gw, model)
		if r.Status != 200 {
			t.Fatalf("request %d: %d %s", i+1, r.Status, r.Body)
		}
		got = append(got, r.Header.Get("X-RateLimit-Remaining"))
	}
	if strings.Join(got, ",") != "3,2,1,0" {
		t.Errorf("X-RateLimit-Remaining over 4 requests with rpm=4 = %v, want [3 2 1 0]", got)
	}
}

// FINDING WS08-2: Retry-After: 1 on an RPM denial, but the sliding window
// only frees a slot after up to 60s.
func TestWS08_RateLimit_RetryAfterHonest(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("ra"), "fake-ok")
	ws08SetLimit(t, gw, 2, 0)
	for i := 0; i < 2; i++ {
		if r := ws08Chat(t, gw, model); r.Status != 200 {
			t.Fatalf("request %d: %d %s", i+1, r.Status, r.Body)
		}
	}
	denied := time.Now()
	r := ws08Chat(t, gw, model)
	ws08AssertRateLimited(t, r, "2")
	ra, _ := strconv.Atoi(r.Header.Get("Retry-After"))
	time.Sleep(time.Duration(ra)*time.Second + 200*time.Millisecond)
	again := ws08Chat(t, gw, model)
	if again.Status != 200 {
		t.Errorf("after honouring Retry-After: %s the request is still %d (Retry-After understates the wait)", r.Header.Get("Retry-After"), again.Status)
	}
	// Recovery: the window must free up within ~60s of the first admitted call.
	Eventually(t, 65*time.Second, func() bool {
		time.Sleep(2 * time.Second)
		return ws08Chat(t, gw, model).Status == 200
	}, "RPM window never recovered")
	t.Logf("recovered %.0fs after the denial", time.Since(denied).Seconds())
}

func TestWS08_RateLimit_Concurrency(t *testing.T) {
	gw := ws08Gateway(t)
	slow := ws08NewSlow(t, 25*time.Second)
	model := ws08Model(t, Uniq("conc"), "slow", slow.URL+"/v1")
	ws08SetLimit(t, gw, 0, 2)

	var wg sync.WaitGroup
	results := make([]Resp, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = gw.DoCtx(context.Background(), http.MethodPost, "/chat/completions", Chat(model, Uniq("ws08-conc"), false))
		}(i)
	}
	Eventually(t, 10*time.Second, func() bool { return slow.Hits() == 2 }, "2 slow requests never reached upstream")

	third := ws08Chat(t, gw, model)
	if third.Elapsed > 3*time.Second {
		t.Errorf("concurrency denial took %s, want immediate", third.Elapsed)
	}
	ws08AssertRateLimited(t, third, "2")
	if slow.Hits() != 2 {
		t.Errorf("denied request reached upstream (hits=%d)", slow.Hits())
	}
	if n, _ := ws08Redis(t).Get(context.Background(), "ratelimit:concurrent:"+gw.Subject).Int(); n != 2 {
		t.Errorf("ratelimit:concurrent = %d while 2 in flight, want 2", n)
	}

	slow.Release()
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i].Status != 200 {
			t.Errorf("in-flight request %d: err=%v status=%d %s", i, errs[i], results[i].Status, results[i].Body)
		}
	}
	// Slots released: key removed, new requests admitted.
	if ex := ws08Redis(t).Exists(context.Background(), "ratelimit:concurrent:"+gw.Subject).Val(); ex != 0 {
		t.Errorf("ratelimit:concurrent key still present after all requests finished")
	}
	if r := ws08Chat(t, gw, model); r.Status != 200 {
		t.Errorf("after release: %d %s", r.Status, r.Body)
	}
}

// FINDING WS08-3: with rpm=0 (unlimited) admitted responses carry
// X-RateLimit-Limit: 0 / X-RateLimit-Remaining: 0, which reads as "exhausted".
func TestWS08_RateLimit_UnlimitedHeaders(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("unl"), "fake-ok")
	ws08SetLimit(t, gw, 0, 0)
	for i := 0; i < 8; i++ {
		r := ws08Chat(t, gw, model)
		if r.Status != 200 {
			t.Fatalf("unlimited request %d: %d %s", i+1, r.Status, r.Body)
		}
		if i == 0 {
			lim, rem := r.Header.Get("X-RateLimit-Limit"), r.Header.Get("X-RateLimit-Remaining")
			if lim == "0" && rem == "0" {
				t.Errorf("unlimited subject gets X-RateLimit-Limit: 0, X-RateLimit-Remaining: 0 on a 200 (clients read this as exhausted); omit the headers instead")
			}
		}
	}
	if ex := ws08Redis(t).Exists(context.Background(), "ratelimit:rpm:"+gw.Subject, "ratelimit:concurrent:"+gw.Subject).Val(); ex != 0 {
		t.Errorf("limiter keys written for an unlimited subject")
	}
}

func TestWS08_RateLimit_UpdateAndDeleteTakeEffect(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("upd"), "fake-ok")
	admin := AdminAPI(t)
	ws08SetLimit(t, gw, 1, 0)
	if r := ws08Chat(t, gw, model); r.Status != 200 {
		t.Fatalf("first: %d %s", r.Status, r.Body)
	}
	ws08AssertRateLimited(t, ws08Chat(t, gw, model), "1")

	// Raise the limit: takes effect on the very next request (no 30s cache lag).
	if r := admin.Patch(t, "/rate-limits/"+gw.RLID, map[string]any{"rpm": 3}); r.Status != 200 {
		t.Fatalf("patch: %d %s", r.Status, r.Body)
	}
	r := ws08Chat(t, gw, model)
	if r.Status != 200 || r.Header.Get("X-RateLimit-Limit") != "3" {
		t.Errorf("right after raising rpm to 3: %d limit=%q (config cache not invalidated?)", r.Status, r.Header.Get("X-RateLimit-Limit"))
	}

	// Delete the subject's row: it falls back to the default config / built-in 60.
	var def struct {
		Items []struct {
			RPM int `json:"rpm"`
		} `json:"items"`
	}
	admin.Get(t, "/rate-limits?subject_id=default").JSON(t, &def)
	wantFallback := "60"
	if len(def.Items) == 1 {
		wantFallback = strconv.Itoa(def.Items[0].RPM)
	}
	if d := admin.Delete(t, "/rate-limits/"+gw.RLID); d.Status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", d.Status, d.Body)
	}
	ws08ResetRedis(t, gw.Subject)
	r = ws08Chat(t, gw, model)
	if r.Status != 200 || r.Header.Get("X-RateLimit-Limit") != wantFallback {
		t.Errorf("after deleting the subject's config: %d limit=%q, want 200 with fallback limit %s", r.Status, r.Header.Get("X-RateLimit-Limit"), wantFallback)
	}
	if g := admin.Get(t, "/rate-limits/"+gw.RLID); g.Status != http.StatusNotFound {
		t.Errorf("GET deleted config: %d", g.Status)
	}
	if d := admin.Delete(t, "/rate-limits/"+gw.RLID); d.Status != http.StatusNotFound {
		t.Errorf("DELETE twice: %d %s", d.Status, d.Body)
	}

	// Recreate the subject's own row (with ws08SetLimit's cleanup restoring defaults to it).
	c := admin.Post(t, "/rate-limits", map[string]any{"name": Uniq("ws08-rl"), "subject_id": gw.Subject, "rpm": ws08BaseRPM, "max_concurrent": ws08BaseMC})
	if c.Status != http.StatusCreated {
		t.Fatalf("recreate: %d %s", c.Status, c.Body)
	}
	ws08PoolMu.Lock()
	ws08Pool.RLID = c.Map(t)["id"].(string)
	ws08PoolMu.Unlock()
	ws08ResetRedis(t, gw.Subject)
	if r := ws08Chat(t, gw, model); r.Header.Get("X-RateLimit-Limit") != strconv.Itoa(ws08BaseRPM) {
		t.Errorf("recreated config not applied immediately: limit=%q", r.Header.Get("X-RateLimit-Limit"))
	}
}

// Rate limits apply on /v1/messages and /v1/responses with their own error envelopes.
// FINDING WS08-4: /v1/messages returns type "api_error" instead of Anthropic's "rate_limit_error".
func TestWS08_RateLimit_OtherEndpoints(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("ep"), "fake-ok")
	ws08SetLimit(t, gw, 1, 0)
	if r := ws08Chat(t, gw, model); r.Status != 200 {
		t.Fatalf("first: %d %s", r.Status, r.Body)
	}
	msg := gw.Post(t, "/messages", map[string]any{"model": model, "max_tokens": 5, "messages": []map[string]any{{"role": "user", "content": "x"}}})
	if msg.Status != 429 || msg.Header.Get("Retry-After") == "" {
		t.Errorf("/v1/messages over limit: %d Retry-After=%q %s", msg.Status, msg.Header.Get("Retry-After"), msg.Body)
	}
	var am struct {
		Type  string `json:"type"`
		Error struct{ Type, Message string }
	}
	msg.JSON(t, &am)
	if am.Type != "error" || am.Error.Type != "rate_limit_error" {
		t.Errorf("/v1/messages 429 envelope = %s, want Anthropic {type:error, error:{type:rate_limit_error}}", msg.Body)
	}
	resp := gw.Post(t, "/responses", map[string]any{"model": model, "input": "x"})
	if resp.Status != 429 || resp.Header.Get("Retry-After") == "" {
		t.Errorf("/v1/responses over limit: %d Retry-After=%q %s", resp.Status, resp.Header.Get("Retry-After"), resp.Body)
	}
}

func TestWS08_RateLimit_Validation(t *testing.T) {
	admin := AdminAPI(t)
	subj := Uniq("ws08-probe")
	cleanup := func() {
		var l struct {
			Items []struct{ ID string } `json:"items"`
		}
		admin.Get(t, "/rate-limits?subject_id="+subj).JSON(t, &l)
		for _, it := range l.Items {
			admin.Delete(t, "/rate-limits/"+it.ID)
		}
	}
	t.Cleanup(cleanup)

	type tc struct {
		name   string
		body   any
		status int
		msg    string
	}
	for _, c := range []tc{
		{"negative rpm", map[string]any{"name": "ws08", "subject_id": subj, "rpm": -1}, 400, "rpm must be non-negative (0 = unlimited)"},
		{"negative concurrency", map[string]any{"name": "ws08", "subject_id": subj, "max_concurrent": -5}, 400, "max_concurrent must be non-negative (0 = unlimited)"},
		{"missing name", map[string]any{"subject_id": subj, "rpm": 1}, 400, "name is required"},
		{"missing subject", map[string]any{"name": "ws08", "rpm": 1}, 400, "subject_id is required"},
		{"fractional rpm", map[string]any{"name": "ws08", "subject_id": subj, "rpm": 1.5}, 400, ""},
		{"string rpm", map[string]any{"name": "ws08", "subject_id": subj, "rpm": "10"}, 400, ""},
		{"malformed json", `{"name":`, 400, ""},
		// FINDING WS08-5: values beyond int32 overflow the INTEGER column → 500.
		{"rpm beyond int32", map[string]any{"name": "ws08", "subject_id": subj, "rpm": int64(2147483648)}, 400, ""},
		{"concurrency beyond int32", map[string]any{"name": "ws08", "subject_id": subj, "max_concurrent": int64(1) << 40}, 400, ""},
		// FINDING WS08-6: whitespace-only names are accepted.
		{"whitespace name", map[string]any{"name": "   ", "subject_id": subj, "rpm": 1}, 400, ""},
		{"whitespace subject", map[string]any{"name": "ws08", "subject_id": "  ", "rpm": 1}, 400, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := admin.Post(t, "/rate-limits", c.body)
			if r.Status == http.StatusCreated {
				admin.Delete(t, "/rate-limits/"+r.Map(t)["id"].(string))
			}
			if r.Status != c.status {
				t.Errorf("POST %v: got %d %s, want %d", c.body, r.Status, r.Body, c.status)
			}
			if c.msg != "" && r.Status == c.status && r.Map(t)["message"] != c.msg {
				t.Errorf("message = %v, want %q", r.Map(t)["message"], c.msg)
			}
			cleanup()
		})
	}

	// Boundary values, unicode name and duplicate subject.
	name := "ws08 límite ✓ " + strings.Repeat("x", 200)
	r := admin.Post(t, "/rate-limits", map[string]any{"name": name, "subject_id": subj, "rpm": 0, "max_concurrent": 2147483647})
	if r.Status != http.StatusCreated {
		t.Fatalf("create rpm=0, max_concurrent=MaxInt32: %d %s", r.Status, r.Body)
	}
	var cfg struct {
		ID, Name, SubjectID string
		RPM                 int `json:"rpm"`
		MaxConcurrent       int `json:"max_concurrent"`
	}
	r.JSON(t, &cfg)
	if cfg.Name != name || cfg.RPM != 0 || cfg.MaxConcurrent != 2147483647 {
		t.Errorf("round trip: %s", r.Body)
	}
	dup := admin.Post(t, "/rate-limits", map[string]any{"name": "ws08-dup", "subject_id": subj, "rpm": 5})
	if dup.Status != http.StatusConflict || dup.Map(t)["message"] != "rate limit config already exists" {
		t.Errorf("duplicate subject: %d %s, want 409 \"rate limit config already exists\"", dup.Status, dup.Body)
	}
	// Defaults when rpm/max_concurrent are omitted.
	omitted := admin.Post(t, "/rate-limits", map[string]any{"name": "ws08-omit", "subject_id": subj + "-o"})
	if omitted.Status == http.StatusCreated {
		om := omitted.Map(t)
		t.Cleanup(func() { admin.Delete(t, "/rate-limits/"+om["id"].(string)) })
		t.Logf("omitted rpm/max_concurrent stored as rpm=%v max_concurrent=%v (0 = unlimited)", om["rpm"], om["max_concurrent"])
	}

	// PATCH validation.
	for _, c := range []tc{
		{"patch negative rpm", map[string]any{"rpm": -1}, 400, "rpm must be non-negative (0 = unlimited)"},
		{"patch negative concurrency", map[string]any{"max_concurrent": -1}, 400, "max_concurrent must be non-negative (0 = unlimited)"},
		{"patch empty name", map[string]any{"name": ""}, 400, "name cannot be empty"},
	} {
		p := admin.Patch(t, "/rate-limits/"+cfg.ID, c.body)
		if p.Status != c.status || p.Map(t)["message"] != c.msg {
			t.Errorf("%s: %d %s, want %d %q", c.name, p.Status, p.Body, c.status, c.msg)
		}
	}
	p := admin.Patch(t, "/rate-limits/"+cfg.ID, map[string]any{"subject_id": "ws08-moved", "rpm": 9})
	if p.Status != 200 || p.Map(t)["subject_id"] != subj || p.Map(t)["rpm"] != float64(9) {
		t.Errorf("PATCH with subject_id must keep the subject and apply rpm: %d %s", p.Status, p.Body)
	}
	if g := admin.Get(t, "/rate-limits?subject_id="+subj); !strings.Contains(string(g.Body), cfg.ID) {
		t.Errorf("filter by subject_id misses the config: %s", g.Body)
	}
	if g := admin.Get(t, "/rate-limits/not-a-uuid"); g.Status != 400 {
		t.Errorf("GET bad id: %d", g.Status)
	}
	missing := "00000000-0000-4000-8000-000000000000"
	if g := admin.Patch(t, "/rate-limits/"+missing, map[string]any{"rpm": 1}); g.Status != 404 {
		t.Errorf("PATCH unknown id: %d %s", g.Status, g.Body)
	}
	if g := admin.Delete(t, "/rate-limits/"+missing); g.Status != 404 {
		t.Errorf("DELETE unknown id: %d %s", g.Status, g.Body)
	}
}
