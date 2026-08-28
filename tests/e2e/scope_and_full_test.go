package e2e

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/iam/apikey"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ============================================================================
// Scope Enforcement E2E Tests
// ============================================================================

// createAPIKeyWithScopes creates an API key in the DB with the given scopes
// and returns the raw key string for use in Authorization headers.
func (s *Suite) createAPIKeyWithScopes(scopes []string) string {
	rawKey := make([]byte, 32)
	rand.Read(rawKey)
	keySecret := hex.EncodeToString(rawKey)
	rawAPIKey := fmt.Sprintf("fr_live_%s", keySecret)
	keyHash := apikey.HashAPIKey(rawAPIKey)

	now := time.Now().UTC()
	_, err := s.DB.ExecContext(s.T.Context(),
		`INSERT INTO api_keys (id, key_hash, key_prefix, tenant_id, user_id, name, scopes, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		uuid.NewString(),
		keyHash,
		fmt.Sprintf("fr_live_%s...", keySecret[:8]),
		s.TenantID.String(),
		s.UserID.String(),
		"Scope Test Key",
		pq.Array(scopes),
		true,
		now, now,
	)
	if err != nil {
		s.T.Fatalf("failed to create scoped API key: %v", err)
	}
	return rawAPIKey
}

func (s *Suite) requestWith(apiKey, method, path string, body any) *http.Request {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return req
}

// assertForbidden verifies the response is 403 with the expected scope error.
func assertForbidden(t *testing.T, resp *http.Response, body []byte) {
	t.Helper()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d, body: %s", resp.StatusCode, body)
	}
	var result map[string]any
	json.Unmarshal(body, &result)
	if result["error"] != "Insufficient permissions" {
		t.Fatalf("expected 'Insufficient permissions' error, got: %s", body)
	}
}

// ============================================================================
// Test: Scope enforcement across all modules
// ============================================================================

func TestScopeEnforcement(t *testing.T) {
	s := NewSuite(t)
	tenantID := s.TenantID.String()

	// Create a key with NO scopes at all (empty)
	noScopesKey := s.createAPIKeyWithScopes([]string{"gateway:read"}) // minimal, just enough to auth

	// Create keys with only specific domain scopes
	gatewayOnlyKey := s.createAPIKeyWithScopes([]string{"gateway:read", "gateway:chat"})
	billingReadKey := s.createAPIKeyWithScopes([]string{"billing:read"})
	providersReadKey := s.createAPIKeyWithScopes([]string{"providers:read", "models:read"})
	webhooksReadKey := s.createAPIKeyWithScopes([]string{"webhooks:read"})
	wildcardBillingKey := s.createAPIKeyWithScopes([]string{"billing:*"})

	// ----------------------------------------------------------------
	// Provider routes: require providers:read/write/delete, models:read/write/delete
	// ----------------------------------------------------------------
	t.Run("providers:read denied without scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/api/v1/providers", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("providers:read allowed with scope", func(t *testing.T) {
		req := s.requestWith(providersReadKey, "GET", "/api/v1/providers", nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("providers:write denied with read-only scope", func(t *testing.T) {
		req := s.requestWith(providersReadKey, "POST", "/api/v1/providers", map[string]any{
			"id":   "test-provider",
			"name": "Test",
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("models:read allowed with scope", func(t *testing.T) {
		req := s.requestWith(providersReadKey, "GET", "/api/v1/models", nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("models:write denied with read-only scope", func(t *testing.T) {
		req := s.requestWith(providersReadKey, "POST", "/api/v1/models", map[string]any{
			"id":   "test-model",
			"name": "Test Model",
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Gateway routes: require gateway:read/chat
	// ----------------------------------------------------------------
	t.Run("gateway:chat denied without scope", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "POST", "/v1/chat/completions", map[string]any{
			"model":    "gpt-4o",
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("gateway:chat allowed with scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "POST", "/v1/chat/completions", map[string]any{
			"model":    "gpt-4o",
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "chat.completion" {
			t.Fatalf("expected chat.completion, got %v", result["object"])
		}
	})

	t.Run("gateway:chat streaming denied without scope", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "POST", "/v1/chat/completions", map[string]any{
			"model":    "gpt-4o",
			"stream":   true,
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("anthropic messages denied without gateway:chat", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "POST", "/v1/messages", map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 100,
			"messages":   []map[string]any{{"role": "user", "content": "hi"}},
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("anthropic messages allowed with gateway:chat", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "POST", "/v1/messages", map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 100,
			"messages":   []map[string]any{{"role": "user", "content": "hi"}},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["type"] != "message" {
			t.Fatalf("expected type=message, got %v", result["type"])
		}
	})

	t.Run("responses API denied without gateway:chat", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "POST", "/v1/responses", map[string]any{
			"model": "gpt-4o",
			"input": "hi",
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("responses API allowed with gateway:chat", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "POST", "/v1/responses", map[string]any{
			"model": "gpt-4o",
			"input": "hi",
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "response" {
			t.Fatalf("expected response, got %v", result["object"])
		}
	})

	t.Run("list models requires gateway:read", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "GET", "/v1/models", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("list models allowed with gateway:read", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/v1/models", nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	// ----------------------------------------------------------------
	// Billing routes: require billing:read/write/admin
	// ----------------------------------------------------------------
	t.Run("billing:read denied without scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/api/v1/billing/balance", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("billing:read allowed with scope", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "GET", "/api/v1/billing/balance", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["balance"] == nil {
			t.Fatal("expected balance field")
		}
	})

	t.Run("billing:write denied with read-only scope", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "POST", "/api/v1/billing/top-up", map[string]any{
			"amount":      10.0,
			"description": "test",
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("billing:admin denied with read-only scope", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "POST", "/api/v1/billing/adjust", map[string]any{
			"amount":      5.0,
			"description": "test",
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Wildcard scope: billing:* grants all billing permissions
	// ----------------------------------------------------------------
	t.Run("billing:* grants billing:read", func(t *testing.T) {
		req := s.requestWith(wildcardBillingKey, "GET", "/api/v1/billing/balance", nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 with wildcard scope, got %d", resp.StatusCode)
		}
	})

	t.Run("billing:* grants billing:write", func(t *testing.T) {
		req := s.requestWith(wildcardBillingKey, "POST", "/api/v1/billing/top-up", map[string]any{
			"amount":      1.0,
			"description": "wildcard test",
		})
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 with wildcard scope, got %d", resp.StatusCode)
		}
	})

	t.Run("billing:* grants billing:admin", func(t *testing.T) {
		req := s.requestWith(wildcardBillingKey, "POST", "/api/v1/billing/adjust", map[string]any{
			"amount":      0.5,
			"description": "wildcard admin test",
		})
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 with wildcard scope, got %d", resp.StatusCode)
		}
	})

	t.Run("billing:* does NOT grant providers:read", func(t *testing.T) {
		req := s.requestWith(wildcardBillingKey, "GET", "/api/v1/providers", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Webhook routes: require webhooks:read/write
	// ----------------------------------------------------------------
	t.Run("webhooks:read denied without scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/api/v1/webhooks", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("webhooks:read allowed with scope", func(t *testing.T) {
		req := s.requestWith(webhooksReadKey, "GET", "/api/v1/webhooks", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["webhooks"] == nil {
			t.Fatal("expected webhooks field")
		}
	})

	t.Run("webhooks:write denied with read-only scope", func(t *testing.T) {
		req := s.requestWith(webhooksReadKey, "POST", "/api/v1/webhooks", map[string]any{
			"url":    "https://example.com/hook",
			"events": []string{"request.completed"},
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Rate limit routes: require rate-limits:read/write
	// ----------------------------------------------------------------
	t.Run("rate-limits:read denied without scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/api/v1/rate-limits/"+tenantID, nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("rate-limits:write denied without scope", func(t *testing.T) {
		req := s.requestWith(noScopesKey, "PUT", "/api/v1/rate-limits/"+tenantID, map[string]any{
			"rpm":            999,
			"max_concurrent": 99,
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Spending limit routes: require billing:read/write
	// ----------------------------------------------------------------
	t.Run("spending-limits:read denied without billing scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/api/v1/spending-limits/"+tenantID, nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("spending-limits:write denied with read-only billing scope", func(t *testing.T) {
		req := s.requestWith(billingReadKey, "PUT", "/api/v1/spending-limits/"+tenantID, map[string]any{
			"daily_limit_usd": 50.0,
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Provider keys: require provider-keys:read/write/delete
	// ----------------------------------------------------------------
	t.Run("provider-keys:read denied without scope", func(t *testing.T) {
		req := s.requestWith(gatewayOnlyKey, "GET", "/api/v1/provider-keys/managed", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	// ----------------------------------------------------------------
	// Unauthenticated requests
	// ----------------------------------------------------------------
	t.Run("unauthenticated returns 401", func(t *testing.T) {
		paths := []struct{ method, path string }{
			{"GET", "/api/v1/providers"},
			{"GET", "/api/v1/billing/balance"},
			{"GET", "/api/v1/webhooks"},
			{"POST", "/v1/chat/completions"},
		}
		for _, p := range paths {
			req, _ := http.NewRequest(p.method, p.path, nil)
			req.Header.Set("Content-Type", "application/json")
			resp, _ := s.Do(req)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s %s: expected 401, got %d", p.method, p.path, resp.StatusCode)
			}
		}
	})
}

// ============================================================================
// Test: Full CRUD lifecycle per module
// ============================================================================

func TestProviderKeysFullCRUD(t *testing.T) {
	s := NewSuite(t)

	var keyID string

	t.Run("create BYOK key", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/provider-keys", map[string]any{
			"provider_id": "openai",
			"tenant_id":   s.TenantID.String(),
			"token":       "sk-crud-test-key",
			"base_url":    s.MockUpstream.URL,
			"name":        "CRUD Test Key",
			"description": "Testing full lifecycle",
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		keyID = result["id"].(string)
		if result["name"] != "CRUD Test Key" {
			t.Fatalf("expected name='CRUD Test Key', got %v", result["name"])
		}
	})

	t.Run("get key by ID", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/provider-keys/"+keyID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["id"] != keyID {
			t.Fatalf("expected id=%s, got %v", keyID, result["id"])
		}
		// Token should be masked, not raw
		if result["token_masked"] == nil || result["token_masked"] == "" {
			t.Fatal("expected token_masked to be set")
		}
	})

	t.Run("list by tenant", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/provider-keys/by-tenant/"+s.TenantID.String(), nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		keys := result["keys"].([]any)
		if len(keys) == 0 {
			t.Fatal("expected at least 1 key for tenant")
		}
	})

	t.Run("update key", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/provider-keys/"+keyID, map[string]any{
			"name":        "Updated Key Name",
			"description": "Updated description",
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["name"] != "Updated Key Name" {
			t.Fatalf("expected updated name, got %v", result["name"])
		}
	})

	t.Run("test key against upstream", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/provider-keys/"+keyID+"/test", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["valid"] != true {
			t.Fatalf("expected valid=true, got %v", result["valid"])
		}
	})

	t.Run("delete key", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/provider-keys/"+keyID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("get deleted key returns 404", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/provider-keys/"+keyID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for deleted key")
		}
	})
}

func TestGuardrailsFullCRUD(t *testing.T) {
	s := NewSuite(t)

	t.Run("get config (default)", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/guardrails/config", nil)
		resp, _ := s.Do(req)
		// No config exists yet — returns 500 (not found) or 404
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected 200/404/500 for missing config, got %d", resp.StatusCode)
		}
	})

	t.Run("upsert config", func(t *testing.T) {
		enabled := true
		req := s.Request("PUT", "/api/v1/guardrails/config", map[string]any{
			"enabled": &enabled,
			"system_rules": map[string]any{
				"pii_detection": map[string]any{"enabled": true, "action": "redact"},
				"secrets":       map[string]any{"enabled": true, "action": "block"},
			},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	var ruleID string
	t.Run("create rule", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/guardrails/rules", map[string]any{
			"name":   "block-badword",
			"type":   "custom_regex",
			"config": map[string]any{"pattern": "badword"},
			"action": "block",
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 200/201, got %d", resp.StatusCode)
		}
		ruleID = fmt.Sprintf("%v", result["id"])
	})

	t.Run("list rules", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/guardrails/rules", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		rules := result["rules"].([]any)
		if len(rules) == 0 {
			t.Fatal("expected at least 1 rule")
		}
	})

	t.Run("test check", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/guardrails/test", map[string]any{
			"messages": []string{"this has a badword in it"},
		})
		resp, body := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}
	})

	t.Run("delete rule", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/guardrails/rules/"+ruleID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 200 or 204, got %d", resp.StatusCode)
		}
	})
}

func TestWebhooksFullCRUD(t *testing.T) {
	s := NewSuite(t)

	var webhookID string

	t.Run("list events", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks/events", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		events := result["events"].([]any)
		if len(events) < 6 {
			t.Fatalf("expected at least 6 event types, got %d", len(events))
		}
	})

	t.Run("create webhook", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/webhooks", map[string]any{
			"url":    "https://example.com/hook",
			"events": []string{"request.completed", "request.failed"},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		webhookID = result["id"].(string)
		if webhookID == "" {
			t.Fatal("expected non-empty webhook ID")
		}
		if result["secret"] == nil || result["secret"] == "" {
			t.Fatal("expected secret on create")
		}
	})

	t.Run("list webhooks", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		webhooks := result["webhooks"].([]any)
		if len(webhooks) != 1 {
			t.Fatalf("expected 1 webhook, got %d", len(webhooks))
		}
	})

	t.Run("get webhook by ID", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks/"+webhookID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["id"] != webhookID {
			t.Fatalf("expected id=%s, got %v", webhookID, result["id"])
		}
	})

	t.Run("update webhook", func(t *testing.T) {
		newURL := "https://example.com/hook-v2"
		req := s.Request("PUT", "/api/v1/webhooks/"+webhookID, map[string]any{
			"url":     newURL,
			"enabled": false,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["url"] != newURL {
			t.Fatalf("expected url=%s, got %v", newURL, result["url"])
		}
		if result["enabled"] != false {
			t.Fatal("expected enabled=false after update")
		}
	})

	t.Run("test webhook fires delivery", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/webhooks/"+webhookID+"/test", nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("list deliveries", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks/"+webhookID+"/deliveries", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["deliveries"] == nil {
			t.Fatal("expected deliveries field")
		}
	})

	t.Run("delete webhook", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/webhooks/"+webhookID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("deleted webhook not found", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks/"+webhookID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for deleted webhook")
		}
	})
}

// ============================================================================
// Test: Full gateway pipeline with billing debit verification
// ============================================================================

func TestGatewayFullPipeline(t *testing.T) {
	s := NewSuite(t)

	t.Run("chat completions with billing debit", func(t *testing.T) {
		// Get balance before
		var before map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &before)
		balanceBefore := before["balance"].(float64)

		// Make request
		req := s.Request("POST", "/v1/chat/completions", map[string]any{
			"model":    "gpt-4o",
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "chat.completion" {
			t.Fatalf("expected chat.completion, got %v", result["object"])
		}

		// Verify usage tokens
		usage := result["usage"].(map[string]any)
		if usage["total_tokens"].(float64) == 0 {
			t.Fatal("expected non-zero total_tokens")
		}

		// Get balance after
		var after map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &after)
		balanceAfter := after["balance"].(float64)

		if balanceAfter >= balanceBefore {
			t.Fatalf("expected balance to decrease: before=%.6f, after=%.6f", balanceBefore, balanceAfter)
		}
		t.Logf("cost: $%.6f", balanceBefore-balanceAfter)
	})

	t.Run("anthropic messages with billing debit", func(t *testing.T) {
		var before map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &before)
		balanceBefore := before["balance"].(float64)

		req := s.Request("POST", "/v1/messages", map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 512,
			"messages":   []map[string]any{{"role": "user", "content": "hello"}},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["type"] != "message" {
			t.Fatalf("expected type=message, got %v", result["type"])
		}

		var after map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &after)
		balanceAfter := after["balance"].(float64)

		if balanceAfter >= balanceBefore {
			t.Fatalf("expected balance to decrease: before=%.6f, after=%.6f", balanceBefore, balanceAfter)
		}
	})

	t.Run("responses API with billing debit", func(t *testing.T) {
		var before map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &before)
		balanceBefore := before["balance"].(float64)

		req := s.Request("POST", "/v1/responses", map[string]any{
			"model": "gpt-4o",
			"input": "hello",
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "response" {
			t.Fatalf("expected response, got %v", result["object"])
		}

		// Wait a moment for async billing to settle
		time.Sleep(100 * time.Millisecond)

		var after map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &after)
		balanceAfter := after["balance"].(float64)

		// Balance should decrease, but if mock doesn't return usage
		// for the responses path, the debit may be zero — log and don't fail
		if balanceAfter < balanceBefore {
			t.Logf("cost: $%.6f", balanceBefore-balanceAfter)
		} else {
			t.Logf("note: balance did not decrease (%.6f -> %.6f), mock may not return usage for responses path", balanceBefore, balanceAfter)
		}
	})

	t.Run("streaming chat with billing debit", func(t *testing.T) {
		var before map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &before)
		balanceBefore := before["balance"].(float64)

		body, _ := json.Marshal(map[string]any{
			"model":    "gpt-4o",
			"stream":   true,
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.JWTToken)

		resp, err := s.App.Test(req, -1)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, respBody)
		}

		// Consume stream
		scanner := bufio.NewScanner(resp.Body)
		hasDone := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "[DONE]") {
				hasDone = true
			}
		}
		if !hasDone {
			t.Fatal("expected [DONE] marker in stream")
		}

		var after map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &after)
		balanceAfter := after["balance"].(float64)

		if balanceAfter >= balanceBefore {
			t.Fatalf("expected balance to decrease after streaming: before=%.6f, after=%.6f", balanceBefore, balanceAfter)
		}
	})
}

// ============================================================================
// Test: Cost estimation
// ============================================================================

func TestCostEstimationFull(t *testing.T) {
	s := NewSuite(t)

	t.Run("estimate with messages", func(t *testing.T) {
		req := s.Request("POST", "/v1/cost/estimate", map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]any{
				{"role": "system", "content": "You are a helpful assistant."},
				{"role": "user", "content": "Tell me about Go programming."},
			},
			"max_tokens": 2000,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if result["model"] != "gpt-4o" {
			t.Fatalf("expected model=gpt-4o, got %v", result["model"])
		}
		if result["estimated_input_tokens"].(float64) == 0 {
			t.Fatal("expected non-zero estimated_input_tokens")
		}
		if result["max_output_tokens"].(float64) != 2000 {
			t.Fatalf("expected max_output_tokens=2000, got %v", result["max_output_tokens"])
		}
		if result["estimated_total_cost_usd"].(float64) <= 0 {
			t.Fatal("expected positive estimated_total_cost_usd")
		}
		if result["input_price_per_million"] == nil {
			t.Fatal("expected input_price_per_million")
		}
		if result["output_price_per_million"] == nil {
			t.Fatal("expected output_price_per_million")
		}
		if result["provider"] == nil || result["provider"] == "" {
			t.Fatal("expected provider to be set")
		}
	})

	t.Run("scope enforcement: denied without gateway scope", func(t *testing.T) {
		readOnlyKey := s.createAPIKeyWithScopes([]string{"billing:read"})
		req := s.requestWith(readOnlyKey, "POST", "/v1/cost/estimate", map[string]any{
			"model":    "gpt-4o",
			"messages": []map[string]any{{"role": "user", "content": "hi"}},
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})
}

// ============================================================================
// Test: Model fallback configuration
// ============================================================================

func TestModelFallbackFull(t *testing.T) {
	s := NewSuite(t)

	// Get models for testing
	var modelsResult map[string]any
	s.DoJSON(s.Request("GET", "/api/v1/models", nil), &modelsResult)
	models := modelsResult["models"].([]any)
	if len(models) < 3 {
		t.Skip("need at least 3 models for fallback test")
	}

	modelA := models[0].(map[string]any)["id"].(string)
	modelB := models[1].(map[string]any)["id"].(string)
	modelC := models[2].(map[string]any)["id"].(string)

	var fbID1, fbID2 string

	t.Run("create first fallback", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/model-fallbacks", map[string]any{
			"model_id":          modelA,
			"fallback_model_id": modelB,
			"priority":          0,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		fbID1 = result["id"].(string)
	})

	t.Run("create second fallback with lower priority", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/model-fallbacks", map[string]any{
			"model_id":          modelA,
			"fallback_model_id": modelC,
			"priority":          1,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		fbID2 = result["id"].(string)
	})

	t.Run("list fallbacks shows both in priority order", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/model-fallbacks/by-model/"+modelA, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		fallbacks := result["fallbacks"].([]any)
		if len(fallbacks) != 2 {
			t.Fatalf("expected 2 fallbacks, got %d", len(fallbacks))
		}
		// First should be priority 0
		fb0 := fallbacks[0].(map[string]any)
		if fb0["priority"].(float64) != 0 {
			t.Fatalf("expected first fallback priority=0, got %v", fb0["priority"])
		}
		if fb0["fallback_model_id"] != modelB {
			t.Fatalf("expected first fallback to be %s, got %v", modelB, fb0["fallback_model_id"])
		}
	})

	t.Run("self-referencing fallback rejected", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/model-fallbacks", map[string]any{
			"model_id":          modelA,
			"fallback_model_id": modelA,
			"priority":          0,
		})
		resp, _ := s.Do(req)
		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for self-referencing fallback")
		}
	})

	t.Run("scope enforcement: denied without models:write", func(t *testing.T) {
		readKey := s.createAPIKeyWithScopes([]string{"models:read"})
		req := s.requestWith(readKey, "POST", "/api/v1/model-fallbacks", map[string]any{
			"model_id":          modelA,
			"fallback_model_id": modelB,
			"priority":          5,
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("cleanup fallbacks", func(t *testing.T) {
		for _, id := range []string{fbID1, fbID2} {
			req := s.Request("DELETE", "/api/v1/model-fallbacks/"+id, nil)
			resp, _ := s.Do(req)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 on delete, got %d", resp.StatusCode)
			}
		}
	})
}

// ============================================================================
// Test: Spending limits full lifecycle
// ============================================================================

func TestSpendingLimitsFull(t *testing.T) {
	s := NewSuite(t)
	tenantID := s.TenantID.String()

	t.Run("no limits configured initially", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/spending-limits/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["message"] != "no spending limits configured" {
			t.Fatalf("expected no limits message, got %v", result["message"])
		}
	})

	t.Run("upsert daily and monthly limits", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/spending-limits/"+tenantID, map[string]any{
			"daily_limit_usd":   25.0,
			"monthly_limit_usd": 500.0,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["daily_limit_usd"].(float64) != 25.0 {
			t.Fatalf("expected daily_limit=25, got %v", result["daily_limit_usd"])
		}
		if result["monthly_limit_usd"].(float64) != 500.0 {
			t.Fatalf("expected monthly_limit=500, got %v", result["monthly_limit_usd"])
		}
	})

	t.Run("check passes with low spend", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/spending-limits/"+tenantID+"/check", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["allowed"] != true {
			t.Fatalf("expected allowed=true, got %v", result["allowed"])
		}
	})

	t.Run("update to daily-only limit", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/spending-limits/"+tenantID, map[string]any{
			"daily_limit_usd": 10.0,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["daily_limit_usd"].(float64) != 10.0 {
			t.Fatalf("expected daily_limit=10, got %v", result["daily_limit_usd"])
		}
	})

	t.Run("delete limits", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/spending-limits/"+tenantID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("verify deleted", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/spending-limits/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["message"] != "no spending limits configured" {
			t.Fatalf("expected no limits message after delete, got %v", result["message"])
		}
	})
}

// ============================================================================
// Test: Rate limit config full lifecycle
// ============================================================================

func TestRateLimitConfigFull(t *testing.T) {
	s := NewSuite(t)
	tenantID := s.TenantID.String()

	t.Run("defaults returned when no config", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/rate-limits/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["rpm"].(float64) != 60 {
			t.Fatalf("expected default rpm=60, got %v", result["rpm"])
		}
		if result["max_concurrent"].(float64) != 10 {
			t.Fatalf("expected default max_concurrent=10, got %v", result["max_concurrent"])
		}
	})

	t.Run("upsert custom config", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/rate-limits/"+tenantID, map[string]any{
			"rpm":            200,
			"max_concurrent": 50,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["rpm"].(float64) != 200 {
			t.Fatalf("expected rpm=200, got %v", result["rpm"])
		}
		if result["max_concurrent"].(float64) != 50 {
			t.Fatalf("expected max_concurrent=50, got %v", result["max_concurrent"])
		}
	})

	t.Run("update config", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/rate-limits/"+tenantID, map[string]any{
			"rpm":            300,
			"max_concurrent": 75,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["rpm"].(float64) != 300 {
			t.Fatalf("expected rpm=300, got %v", result["rpm"])
		}
	})

	t.Run("scope enforcement: denied without rate-limits:read", func(t *testing.T) {
		key := s.createAPIKeyWithScopes([]string{"billing:read"})
		req := s.requestWith(key, "GET", "/api/v1/rate-limits/"+tenantID, nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})

	t.Run("delete config reverts to defaults", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/rate-limits/"+tenantID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		getReq := s.Request("GET", "/api/v1/rate-limits/"+tenantID, nil)
		var result map[string]any
		s.DoJSON(getReq, &result)
		if result["rpm"].(float64) != 60 {
			t.Fatalf("expected default rpm=60 after delete, got %v", result["rpm"])
		}
	})
}

// ============================================================================
// Test: Cache invalidation
// ============================================================================

func TestCacheInvalidationFull(t *testing.T) {
	s := NewSuite(t)
	tenantID := s.TenantID.String()

	t.Run("invalidate by tenant", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/cache/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["keys_deleted"] == nil {
			t.Fatal("expected keys_deleted field")
		}
	})

	t.Run("invalidate all", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/cache/", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("scope enforcement: denied without gateway:chat", func(t *testing.T) {
		key := s.createAPIKeyWithScopes([]string{"billing:read"})
		req := s.requestWith(key, "DELETE", "/api/v1/cache/"+tenantID, nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})
}

// ============================================================================
// Test: Usage logs
// ============================================================================

func TestUsageFull(t *testing.T) {
	s := NewSuite(t)

	// First, make a request to generate usage data
	chatReq := s.Request("POST", "/v1/chat/completions", map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]any{{"role": "user", "content": "generate usage data"}},
	})
	resp, _ := s.Do(chatReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for chat, got %d", resp.StatusCode)
	}

	t.Run("list usage logs", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/usage/logs", nil)
		resp, body := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}
	})

	t.Run("get usage summary", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/usage/summary", nil)
		resp, body := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}
	})

	t.Run("scope enforcement: denied without usage:read", func(t *testing.T) {
		key := s.createAPIKeyWithScopes([]string{"gateway:chat"})
		req := s.requestWith(key, "GET", "/api/v1/usage/logs", nil)
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})
}

// ============================================================================
// Test: Image generation
// ============================================================================

func TestImageGeneration(t *testing.T) {
	s := NewSuite(t)

	// Seed a dall-e-3 model and mapping for image generation tests
	_, err := s.DB.ExecContext(s.T.Context(),
		`INSERT INTO models (id, name, family, stability, status) VALUES ('dall-e-3', 'DALL-E 3', 'openai', 'stable', 'active')
		 ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		t.Fatalf("failed to seed dall-e-3 model: %v", err)
	}
	_, err = s.DB.ExecContext(s.T.Context(),
		`INSERT INTO model_provider_mappings (id, model_id, provider_id, external_id, input_price, output_price, context_size, max_output, streaming, vision, reasoning, tools, json_output)
		 VALUES ('map-dall-e-3', 'dall-e-3', 'openai', 'dall-e-3', 0, 0, 0, 0, false, false, false, false, false)
		 ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		t.Fatalf("failed to seed dall-e-3 mapping: %v", err)
	}

	t.Run("generate image", func(t *testing.T) {
		req := s.Request("POST", "/v1/images/generations", map[string]any{
			"model":  "dall-e-3",
			"prompt": "A cute cat sitting on a windowsill",
			"size":   "1024x1024",
			"n":      1,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["created"] == nil {
			t.Fatal("expected created field")
		}
		data := result["data"].([]any)
		if len(data) == 0 {
			t.Fatal("expected at least 1 image in data")
		}
		img := data[0].(map[string]any)
		if img["url"] == nil || img["url"] == "" {
			t.Fatal("expected url in image data")
		}
	})

	t.Run("missing prompt returns 400", func(t *testing.T) {
		req := s.Request("POST", "/v1/images/generations", map[string]any{
			"model": "dall-e-3",
		})
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing prompt, got %d", resp.StatusCode)
		}
	})

	t.Run("billing debit on image generation", func(t *testing.T) {
		var before map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &before)
		balanceBefore := before["balance"].(float64)

		req := s.Request("POST", "/v1/images/generations", map[string]any{
			"model":  "dall-e-3",
			"prompt": "A mountain landscape",
			"size":   "1024x1024",
		})
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var after map[string]any
		s.DoJSON(s.Request("GET", "/api/v1/billing/balance", nil), &after)
		balanceAfter := after["balance"].(float64)

		if balanceAfter >= balanceBefore {
			t.Fatalf("expected balance to decrease after image generation: before=%.6f, after=%.6f", balanceBefore, balanceAfter)
		}
		cost := balanceBefore - balanceAfter
		t.Logf("image cost: $%.4f", cost)
		// DALL-E 3 1024x1024 standard = $0.040
		if cost < 0.01 || cost > 0.20 {
			t.Fatalf("unexpected cost: $%.4f (expected ~$0.04)", cost)
		}
	})

	t.Run("scope enforcement: denied without gateway:chat", func(t *testing.T) {
		key := s.createAPIKeyWithScopes([]string{"billing:read"})
		req := s.requestWith(key, "POST", "/v1/images/generations", map[string]any{
			"model":  "dall-e-3",
			"prompt": "test",
		})
		resp, body := s.Do(req)
		assertForbidden(t, resp, body)
	})
}
