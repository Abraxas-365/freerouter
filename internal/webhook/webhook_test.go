package webhook_test

import (
	"testing"

	"github.com/Abraxas-365/freerouter/internal/webhook"
)

// ── ValidateURL ─────────────────────────────────────────────────────

func TestValidateURL_AcceptsPlainHTTPS(t *testing.T) {
	if err := webhook.ValidateURL("https://example.com/webhook"); err != nil {
		t.Fatalf("expected valid URL, got error: %v", err)
	}
}

func TestValidateURL_AcceptsPlainHTTP(t *testing.T) {
	if err := webhook.ValidateURL("http://example.com/webhook"); err != nil {
		t.Fatalf("expected valid URL, got error: %v", err)
	}
}

func TestValidateURL_RejectsNonHTTPScheme(t *testing.T) {
	cases := []string{
		"ftp://example.com/webhook",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"gopher://example.com",
	}
	for _, raw := range cases {
		if err := webhook.ValidateURL(raw); err == nil {
			t.Errorf("expected scheme rejection for %q", raw)
		}
	}
}

func TestValidateURL_RejectsEmptyHostname(t *testing.T) {
	if err := webhook.ValidateURL("https:///path-only"); err == nil {
		t.Fatalf("expected rejection for missing hostname")
	}
}

func TestValidateURL_RejectsEmbeddedCredentials(t *testing.T) {
	if err := webhook.ValidateURL("https://user:pass@example.com/webhook"); err == nil {
		t.Fatalf("expected rejection for embedded credentials")
	}
}

func TestValidateURL_RejectsFragment(t *testing.T) {
	if err := webhook.ValidateURL("https://example.com/webhook#section"); err == nil {
		t.Fatalf("expected rejection for fragment")
	}
}

func TestValidateURL_RejectsMalformedURL(t *testing.T) {
	if err := webhook.ValidateURL("not a url at all"); err == nil {
		t.Fatalf("expected rejection for malformed URL")
	}
}

func TestValidateURL_AllowsQueryString(t *testing.T) {
	if err := webhook.ValidateURL("https://example.com/webhook?token=abc"); err != nil {
		t.Fatalf("expected query strings to be allowed, got: %v", err)
	}
}

// ── Event types ─────────────────────────────────────────────────────

func TestIsValidEvent(t *testing.T) {
	for _, e := range webhook.AllEvents() {
		if !webhook.IsValidEvent(e) {
			t.Errorf("expected %q from AllEvents() to be valid", e)
		}
	}
	if webhook.IsValidEvent("not.a.real.event") {
		t.Fatalf("expected unknown event to be invalid")
	}
	if webhook.IsValidEvent("") {
		t.Fatalf("expected empty event to be invalid")
	}
}

func TestAllEvents_NoBillingEvents(t *testing.T) {
	// v2 is open-source and has no billing/spending features; ensure no
	// billing-related event slipped back in.
	for _, e := range webhook.AllEvents() {
		if e == "spending.warning" || e == "spending.exceeded" {
			t.Fatalf("unexpected billing event present: %q", e)
		}
	}
}

// ── CreateWebhook.Validate ───────────────────────────────────────────

func TestCreateWebhook_Validate_OK(t *testing.T) {
	cmd := webhook.CreateWebhook{
		URL:    "https://example.com/hook",
		Events: []string{webhook.EventRequestCompleted},
	}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("expected valid command, got error: %v", err)
	}
}

func TestCreateWebhook_Validate_RejectsBadURL(t *testing.T) {
	cmd := webhook.CreateWebhook{
		URL:    "not-a-url",
		Events: []string{webhook.EventRequestCompleted},
	}
	if err := cmd.Validate(); err == nil {
		t.Fatalf("expected validation error for bad URL")
	}
}

func TestCreateWebhook_Validate_RejectsEmptyEvents(t *testing.T) {
	cmd := webhook.CreateWebhook{URL: "https://example.com/hook"}
	if err := cmd.Validate(); err == nil {
		t.Fatalf("expected validation error for empty events")
	}
}

func TestCreateWebhook_Validate_RejectsUnknownEvent(t *testing.T) {
	cmd := webhook.CreateWebhook{
		URL:    "https://example.com/hook",
		Events: []string{"totally.bogus.event"},
	}
	if err := cmd.Validate(); err == nil {
		t.Fatalf("expected validation error for unknown event")
	}
}

// ── UpdateWebhook.Validate ───────────────────────────────────────────

func TestUpdateWebhook_Validate_AllNilOK(t *testing.T) {
	cmd := webhook.UpdateWebhook{}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("expected no-op update to be valid, got: %v", err)
	}
}

func TestUpdateWebhook_Validate_RejectsBadURL(t *testing.T) {
	bad := "javascript:alert(1)"
	cmd := webhook.UpdateWebhook{URL: &bad}
	if err := cmd.Validate(); err == nil {
		t.Fatalf("expected validation error for bad URL")
	}
}

func TestUpdateWebhook_Validate_RejectsUnknownEvent(t *testing.T) {
	cmd := webhook.UpdateWebhook{Events: []string{"unknown.event"}}
	if err := cmd.Validate(); err == nil {
		t.Fatalf("expected validation error for unknown event")
	}
}

func TestUpdateWebhook_Validate_AcceptsValidPartialUpdate(t *testing.T) {
	enabled := false
	cmd := webhook.UpdateWebhook{Enabled: &enabled}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("expected valid partial update, got: %v", err)
	}
}
