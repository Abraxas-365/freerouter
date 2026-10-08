//go:build e2e

package api

import (
	"net/http"
	"testing"
)

// TestHarnessSmoke proves the harness wiring; workstreams add their own files.
func TestHarnessSmoke(t *testing.T) {
	f := FX(t)

	t.Run("health is public", func(t *testing.T) {
		r := Anon(f.URLs.Server).Get(t, "/health")
		if r.Status != 200 {
			t.Fatalf("health: %d %s", r.Status, r.Body)
		}
	})

	t.Run("gateway key can chat through fakellm", func(t *testing.T) {
		ResetFakeLLM(t)
		gw := Bearer(f.URLs.Gateway, f.Personas["gw_key"].Secret)
		r := gw.Post(t, "/chat/completions", Chat("e2e-ok", "hi", false))
		if r.Status != 200 {
			t.Fatalf("chat: %d %s", r.Status, r.Body)
		}
		if got := FakeLLMRequests(t); len(got) != 1 || got[0]["model"] != "fake-ok" {
			t.Fatalf("fakellm recorded %v", got)
		}
	})

	t.Run("gateway key is forbidden on management API", func(t *testing.T) {
		r := Bearer(f.URLs.API, f.Personas["gw_key"].Secret).Get(t, "/providers")
		if r.Status != http.StatusForbidden {
			t.Fatalf("want 403, got %d %s", r.Status, r.Body)
		}
	})

	t.Run("user JWT works on management API", func(t *testing.T) {
		r := AsUser(t, "viewer").Get(t, "/providers")
		if r.Status != 200 {
			t.Fatalf("viewer list providers: %d %s", r.Status, r.Body)
		}
	})
}
