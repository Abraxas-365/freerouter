//go:build e2e

package api

// WS-08 webhooks: CRUD/validation, HMAC signing, event filtering, test-fire,
// retry/backoff against the sink's /hook/fail/<n>/ paths, disabled webhooks,
// delete cascade and secret exposure. Events are global (every worker's
// gateway traffic fires them), so deliveries are matched by sink path AND
// by the WS-08 model name inside the payload.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type ws08Payload struct {
	ID        string         `json:"id"`
	Event     string         `json:"event"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// ws08SinkFor returns sink deliveries on path whose payload data.model == model.
func ws08SinkFor(t *testing.T, path, model string) []ws08Delivery {
	t.Helper()
	var out []ws08Delivery
	for _, d := range ws08Sink(t, path) {
		var p ws08Payload
		_ = json.Unmarshal(d.Body, &p)
		if model == "" || p.Data["model"] == model {
			out = append(out, d)
		}
	}
	return out
}

func ws08Sign(body []byte, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

type ws08DeliveryRow struct {
	ID          string     `json:"id"`
	WebhookID   string     `json:"webhook_id"`
	EventType   string     `json:"event_type"`
	Payload     string     `json:"payload"`
	Status      string     `json:"status"`
	StatusCode  *int       `json:"status_code"`
	Attempts    int        `json:"attempts"`
	LastError   *string    `json:"last_error"`
	NextRetryAt *time.Time `json:"next_retry_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

func ws08DeliveryRows(t *testing.T, hookID string) []ws08DeliveryRow {
	t.Helper()
	var page struct {
		Items []ws08DeliveryRow `json:"items"`
	}
	r := AdminAPI(t).Get(t, "/webhooks/"+hookID+"/deliveries?limit=100")
	if r.Status != 200 {
		t.Fatalf("list deliveries: %d %s", r.Status, r.Body)
	}
	r.JSON(t, &page)
	return page.Items
}

func TestWS08_Webhook_SignedDeliveryAndEventFilter(t *testing.T) {
	gw := ws08Gateway(t)
	okModel := ws08Model(t, Uniq("whok"), "fake-ok")
	failModel := ws08Model(t, Uniq("whfail"), "fake-400")
	path := "/hook/" + Uniq("ws08-signed")
	hook := ws08Webhook(t, path, "request.failed")
	if !strings.HasPrefix(hook.Secret, "whsec_") || len(hook.Secret) != len("whsec_")+48 {
		t.Errorf("secret format = %q", hook.Secret)
	}

	if r := gw.Post(t, "/chat/completions", Chat(okModel, Uniq("ws08-wh"), false)); r.Status != 200 {
		t.Fatalf("ok request: %d %s", r.Status, r.Body)
	}
	if r := gw.Post(t, "/chat/completions", Chat(failModel, Uniq("ws08-wh"), false)); r.Status < 400 {
		t.Fatalf("failing request returned %d", r.Status)
	}
	var got []ws08Delivery
	Eventually(t, 15*time.Second, func() bool {
		got = ws08SinkFor(t, path, failModel)
		return len(got) >= 1
	}, "request.failed never delivered")
	d := got[0]
	if d.Headers["Content-Type"] != "application/json" || d.Headers["X-Webhook-Event"] != "request.failed" {
		t.Errorf("headers = %v", d.Headers)
	}
	if want := ws08Sign(d.Body, hook.Secret); d.Headers["X-Webhook-Signature"] != want {
		t.Errorf("X-Webhook-Signature = %q, want %q", d.Headers["X-Webhook-Signature"], want)
	}
	if ws08Sign(d.Body, hook.Secret+"x") == d.Headers["X-Webhook-Signature"] {
		t.Errorf("signature does not depend on secret")
	}
	var p ws08Payload
	if err := json.Unmarshal(d.Body, &p); err != nil {
		t.Fatal(err)
	}
	if p.Event != "request.failed" || p.ID == "" || time.Since(p.Timestamp) > time.Minute || p.Data["status_code"] != float64(400) || p.Data["error"] == nil || p.Data["provider"] == nil {
		t.Errorf("payload = %s", d.Body)
	}
	// Event filtering: no request.completed for the ok model on this path.
	time.Sleep(2 * time.Second)
	if n := len(ws08SinkFor(t, path, okModel)); n != 0 {
		t.Errorf("webhook subscribed to request.failed received %d request.completed deliveries", n)
	}
	for _, dd := range ws08Sink(t, path) {
		if dd.Headers["X-Webhook-Event"] != "request.failed" {
			t.Errorf("unsubscribed event delivered: %s", dd.Headers["X-Webhook-Event"])
		}
	}
	// Delivery recorded as success.
	Eventually(t, 10*time.Second, func() bool {
		for _, row := range ws08DeliveryRows(t, hook.ID) {
			if strings.Contains(row.Payload, failModel) {
				return row.Status == "success" && row.Attempts == 1 && row.StatusCode != nil && *row.StatusCode == 200 && row.CompletedAt != nil
			}
		}
		return false
	}, "delivery row not success/attempts=1/200")
}

func TestWS08_Webhook_RetryBackoff(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("whretry"), "fake-400")
	name := Uniq("ws08-retry")
	path := "/hook/fail/2/" + name
	hook := ws08Webhook(t, path, "request.failed")
	if r := gw.Post(t, "/chat/completions", Chat(model, Uniq("ws08-wh"), false)); r.Status < 400 {
		t.Fatalf("failing request returned %d", r.Status)
	}
	// Attempt 1 fails → pending with next_retry_at ≈ +30s.
	var row ws08DeliveryRow
	Eventually(t, 15*time.Second, func() bool {
		for _, r := range ws08DeliveryRows(t, hook.ID) {
			if strings.Contains(r.Payload, model) && r.Attempts == 1 {
				row = r
				return true
			}
		}
		return false
	}, "first attempt not recorded")
	if row.Status != "pending" || row.StatusCode == nil || *row.StatusCode != 500 || row.LastError == nil || *row.LastError != "upstream returned status 500" || row.NextRetryAt == nil {
		t.Errorf("after 1st failure: %+v", row)
	}
	// Retries: 500, then 200 on the 3rd attempt (backoff 30s, 60s; worker polls every 10s).
	Eventually(t, 150*time.Second, func() bool {
		time.Sleep(3 * time.Second)
		for _, r := range ws08DeliveryRows(t, hook.ID) {
			if strings.Contains(r.Payload, model) {
				row = r
			}
		}
		return row.Status == "success"
	}, "delivery never succeeded after retries")
	if row.Attempts != 3 || row.StatusCode == nil || *row.StatusCode != 200 {
		t.Errorf("final delivery row: %+v", row)
	}
	got := ws08SinkFor(t, path, model)
	if len(got) != 3 {
		t.Fatalf("sink saw %d attempts, want 3", len(got))
	}
	var ids []string
	for i, d := range got {
		var p ws08Payload
		_ = json.Unmarshal(d.Body, &p)
		ids = append(ids, p.ID)
		// The sink re-encodes bodies compactly; attempt 1 is sent compact, but
		// retries send the JSONB-rendered payload (spaces), so verify those
		// against the stored payload text, which is what was signed and sent.
		want := ws08Sign(d.Body, hook.Secret)
		if i > 0 {
			want = ws08Sign([]byte(row.Payload), hook.Secret)
		}
		if d.Headers["X-Webhook-Signature"] != want {
			t.Errorf("attempt %d signature %s does not verify", i+1, d.Headers["X-Webhook-Signature"])
		}
	}
	if ids[0] != ids[1] || ids[1] != ids[2] {
		t.Errorf("payload id changes across retries (receivers can't dedupe): %v", ids)
	}
	gap1, gap2 := got[1].At.Sub(got[0].At), got[2].At.Sub(got[1].At)
	if gap1 < 29*time.Second || gap2 < 59*time.Second || gap2 <= gap1 {
		t.Errorf("backoff gaps = %s, %s; want >=30s then >=60s (linear backoff)", gap1, gap2)
	}
}

func TestWS08_Webhook_DisabledGetsNothing(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("whdis"), "fake-400")
	path := "/hook/" + Uniq("ws08-disabled")
	hook := ws08Webhook(t, path, "request.failed")
	p := AdminAPI(t).Patch(t, "/webhooks/"+hook.ID, map[string]any{"enabled": false})
	if p.Status != 200 || p.Map(t)["enabled"] != false {
		t.Fatalf("disable: %d %s", p.Status, p.Body)
	}
	if strings.Contains(string(p.Body), "whsec_") {
		t.Errorf("PATCH response exposes the secret: %s", p.Body)
	}
	gw.Post(t, "/chat/completions", Chat(model, Uniq("ws08-wh"), false))
	time.Sleep(4 * time.Second)
	if n := len(ws08Sink(t, path)); n != 0 {
		t.Errorf("disabled webhook received %d deliveries", n)
	}
	if rows := ws08DeliveryRows(t, hook.ID); len(rows) != 0 {
		t.Errorf("disabled webhook has %d delivery rows", len(rows))
	}
	// Re-enable → deliveries resume.
	AdminAPI(t).Patch(t, "/webhooks/"+hook.ID, map[string]any{"enabled": true})
	gw.Post(t, "/chat/completions", Chat(model, Uniq("ws08-wh"), false))
	Eventually(t, 15*time.Second, func() bool { return len(ws08SinkFor(t, path, model)) == 1 }, "re-enabled webhook got nothing")
}

// FINDING WS08-12: test-fire responds 200 "test event dispatched" but nothing
// is ever delivered: Fire("webhook.test") only targets webhooks subscribed to
// "webhook.test", which is not a valid event to subscribe to.
func TestWS08_Webhook_TestFire(t *testing.T) {
	admin := AdminAPI(t)
	path := "/hook/" + Uniq("ws08-testfire")
	hook := ws08Webhook(t, path, "request.failed")
	r := admin.Post(t, "/webhooks/"+hook.ID+"/test", nil)
	if r.Status != 200 || r.Map(t)["message"] != "test event dispatched" {
		t.Fatalf("test: %d %s", r.Status, r.Body)
	}
	if c := admin.Post(t, "/webhooks", map[string]any{"url": FX(t).URLs.WebhookSink + path, "events": []string{"webhook.test"}}); c.Status != 400 {
		t.Errorf("subscribing to webhook.test: %d %s", c.Status, c.Body)
		if c.Status == 201 {
			admin.Delete(t, "/webhooks/"+c.Map(t)["id"].(string))
		}
	}
	if g := admin.Post(t, "/webhooks/00000000-0000-4000-8000-000000000000/test", nil); g.Status != 404 {
		t.Errorf("test unknown webhook: %d", g.Status)
	}
	if g := admin.Post(t, "/webhooks/nope/test", nil); g.Status != 400 {
		t.Errorf("test bad id: %d", g.Status)
	}
	// Other workers' gateway failures also fire request.failed at this hook;
	// only webhook.test deliveries count.
	testEvents := func() []ws08Delivery {
		var out []ws08Delivery
		for _, d := range ws08Sink(t, path) {
			if d.Headers["X-Webhook-Event"] == "webhook.test" {
				out = append(out, d)
			}
		}
		return out
	}
	var got []ws08Delivery
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && len(got) == 0 {
		time.Sleep(time.Second)
		got = testEvents()
	}
	if len(got) != 1 {
		t.Fatalf("test-fire delivered %d events to the webhook, want 1", len(got))
	}
	var p ws08Payload
	_ = json.Unmarshal(got[0].Body, &p)
	if p.Event != "webhook.test" || p.Data["webhook_id"] != hook.ID || got[0].Headers["X-Webhook-Signature"] != ws08Sign(got[0].Body, hook.Secret) {
		t.Errorf("test delivery: %v %s", got[0].Headers, got[0].Body)
	}
}

func TestWS08_Webhook_Validation(t *testing.T) {
	admin := AdminAPI(t)
	sink := FX(t).URLs.WebhookSink
	urlMsg := "webhook url must be http(s), without credentials or a fragment"
	cases := []struct {
		name   string
		body   any
		status int
		msg    string
	}{
		{"ftp scheme", map[string]any{"url": "ftp://example.com/x", "events": []string{"request.failed"}}, 400, urlMsg},
		{"javascript scheme", map[string]any{"url": "javascript:alert(1)", "events": []string{"request.failed"}}, 400, urlMsg},
		{"file scheme", map[string]any{"url": "file:///etc/passwd", "events": []string{"request.failed"}}, 400, urlMsg},
		{"no host", map[string]any{"url": "http:///hook", "events": []string{"request.failed"}}, 400, urlMsg},
		{"relative", map[string]any{"url": "/hook/x", "events": []string{"request.failed"}}, 400, urlMsg},
		{"credentials", map[string]any{"url": "http://u:p@localhost:29200/hook/x", "events": []string{"request.failed"}}, 400, urlMsg},
		{"fragment", map[string]any{"url": sink + "/hook/x#frag", "events": []string{"request.failed"}}, 400, urlMsg},
		{"empty url", map[string]any{"url": "", "events": []string{"request.failed"}}, 400, urlMsg},
		{"no events", map[string]any{"url": sink + "/hook/x", "events": []string{}}, 400, "at least one event is required"},
		{"unknown event", map[string]any{"url": sink + "/hook/x", "events": []string{"request.bogus"}}, 400, "invalid event type: request.bogus"},
		{"malformed json", `{"url":`, 400, ""},
		// FINDING WS08-13: duplicate events are stored as-is.
		{"duplicate events", map[string]any{"url": sink + "/hook/" + Uniq("ws08-dup"), "events": []string{"request.failed", "request.failed"}}, 400, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := admin.Post(t, "/webhooks", c.body)
			if r.Status == http.StatusCreated {
				admin.Delete(t, "/webhooks/"+r.Map(t)["id"].(string))
			}
			if r.Status != c.status {
				t.Errorf("got %d %s, want %d", r.Status, r.Body, c.status)
			} else if c.msg != "" && r.Map(t)["message"] != c.msg {
				t.Errorf("message = %v, want %q", r.Map(t)["message"], c.msg)
			}
		})
	}

	hook := ws08Webhook(t, "/hook/"+Uniq("ws08-val"), "request.failed", "request.completed")
	// FINDING WS08-14: PATCH {"events":[]} leaves a webhook subscribed to nothing.
	if p := admin.Patch(t, "/webhooks/"+hook.ID, map[string]any{"events": []string{}}); p.Status != 400 {
		t.Errorf("PATCH events=[]: %d %s, want 400 \"at least one event is required\"", p.Status, p.Body)
	}
	for _, bad := range []map[string]any{{"url": "ftp://x"}, {"events": []string{"nope"}}} {
		if p := admin.Patch(t, "/webhooks/"+hook.ID, bad); p.Status != 400 {
			t.Errorf("PATCH %v: %d %s", bad, p.Status, p.Body)
		}
	}
	if g := admin.Get(t, "/webhooks/nope"); g.Status != 400 || g.Map(t)["message"] != "invalid webhook id" {
		t.Errorf("bad id: %d %s", g.Status, g.Body)
	}
	missing := "00000000-0000-4000-8000-000000000000"
	for _, r := range []Resp{admin.Get(t, "/webhooks/"+missing), admin.Patch(t, "/webhooks/"+missing, map[string]any{"enabled": true}), admin.Delete(t, "/webhooks/"+missing), admin.Get(t, "/webhooks/"+missing+"/deliveries")} {
		if r.Status != 404 {
			t.Errorf("unknown webhook id: %d %s", r.Status, r.Body)
		}
	}
	ev := admin.Get(t, "/webhooks/events")
	if ev.Status != 200 || string(ev.Body) != `{"events":["request.completed","request.failed","key.health_degraded","key.blacklisted"]}` {
		t.Errorf("events list: %d %s", ev.Status, ev.Body)
	}
}

// Documents WEBHOOK_ALLOW_PRIVATE=true (e2e stack): private/loopback/link-local
// targets are accepted at create time (the guard is dial-time only and disabled).
func TestWS08_Webhook_PrivateTargetsAllowedInE2E(t *testing.T) {
	admin := AdminAPI(t)
	for _, u := range []string{"http://127.0.0.1:1/ws08", "http://169.254.169.254/latest/meta-data", "http://10.0.0.1/ws08", "http://[::1]:1/ws08"} {
		r := admin.Post(t, "/webhooks", map[string]any{"url": u, "events": []string{"key.blacklisted"}})
		if r.Status == http.StatusCreated {
			id := r.Map(t)["id"].(string)
			admin.Delete(t, "/webhooks/"+id)
		}
		if r.Status != http.StatusCreated {
			t.Errorf("%s: %d %s (expected accepted with WEBHOOK_ALLOW_PRIVATE=true)", u, r.Status, r.Body)
		}
	}
}

func TestWS08_Webhook_SecretAndDeleteCascade(t *testing.T) {
	gw := ws08Gateway(t)
	model := ws08Model(t, Uniq("whdel"), "fake-400")
	admin := AdminAPI(t)
	path := "/hook/" + Uniq("ws08-del")
	hook := ws08Webhook(t, path, "request.failed")
	for _, r := range []Resp{admin.Get(t, "/webhooks/"+hook.ID), admin.Get(t, "/webhooks?limit=100")} {
		if r.Status != 200 || strings.Contains(string(r.Body), hook.Secret) || strings.Contains(string(r.Body), `"secret"`) {
			t.Errorf("secret exposed after create: %s", r.Body)
		}
	}
	gw.Post(t, "/chat/completions", Chat(model, Uniq("ws08-wh"), false))
	Eventually(t, 15*time.Second, func() bool { return len(ws08DeliveryRows(t, hook.ID)) >= 1 }, "no delivery row")
	for _, d := range ws08DeliveryRows(t, hook.ID) {
		if strings.Contains(d.Payload, hook.Secret) {
			t.Errorf("delivery payload contains the secret")
		}
	}
	if d := admin.Delete(t, "/webhooks/"+hook.ID); d.Status != http.StatusNoContent {
		t.Fatalf("delete: %d %s", d.Status, d.Body)
	}
	if g := admin.Get(t, "/webhooks/"+hook.ID+"/deliveries"); g.Status != 404 {
		t.Errorf("deliveries after delete: %d", g.Status)
	}
	var n int
	if err := ws08Postgres(t).QueryRow(`SELECT count(*) FROM webhook_deliveries WHERE webhook_id = $1`, hook.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d delivery rows survive webhook delete", n)
	}
	before := len(ws08Sink(t, path))
	gw.Post(t, "/chat/completions", Chat(model, Uniq("ws08-wh"), false))
	time.Sleep(3 * time.Second)
	if after := len(ws08Sink(t, path)); after != before {
		t.Errorf("deleted webhook still delivered")
	}
	// Secret rotation: no endpoint exists.
	if r := admin.Post(t, "/webhooks/"+hook.ID+"/rotate-secret", nil); r.Status != 404 && r.Status != 405 {
		t.Logf("rotate-secret: %d", r.Status)
	}
}

// Runs last (alphabetical file + name order): revoke the WS-08 service account.
func TestWS08_ZZZ_Cleanup(t *testing.T) { ws08CleanupPool(t) }
