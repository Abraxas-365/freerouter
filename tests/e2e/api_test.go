package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// ============================================================================
// Provider Registry E2E Tests
// ============================================================================

func TestProviderRegistry(t *testing.T) {
	s := NewSuite(t)

	t.Run("list providers", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/providers", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		providers := result["providers"].([]any)
		if len(providers) == 0 {
			t.Fatal("expected seeded providers, got empty list")
		}

		// Verify known providers exist
		providerIDs := make(map[string]bool)
		for _, p := range providers {
			providerIDs[p.(map[string]any)["id"].(string)] = true
		}
		for _, expected := range []string{"openai", "anthropic", "google-ai-studio", "mistral", "deepseek", "xai"} {
			if !providerIDs[expected] {
				t.Errorf("expected provider %s in list", expected)
			}
		}
	})

	t.Run("list models", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/models", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		models := result["models"].([]any)
		if len(models) == 0 {
			t.Fatal("expected seeded models, got empty list")
		}
	})

	t.Run("list mappings by model", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/models/gpt-4o/mappings", nil)
		resp, body := s.Do(req)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}
	})
}

// ============================================================================
// GET /v1/models E2E Test
// ============================================================================

func TestListModelsEndpoint(t *testing.T) {
	s := NewSuite(t)

	t.Run("returns OpenAI-compatible model list", func(t *testing.T) {
		req := s.Request("GET", "/v1/models", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "list" {
			t.Fatalf("expected object=list, got %v", result["object"])
		}

		data := result["data"].([]any)
		if len(data) == 0 {
			t.Fatal("expected models in data, got empty")
		}

		// Verify model format
		firstModel := data[0].(map[string]any)
		if firstModel["object"] != "model" {
			t.Fatalf("expected object=model, got %v", firstModel["object"])
		}
		if firstModel["id"] == nil || firstModel["id"] == "" {
			t.Fatal("expected model id to be set")
		}
		if firstModel["owned_by"] == nil || firstModel["owned_by"] == "" {
			t.Fatal("expected owned_by to be set")
		}
	})

	t.Run("requires auth", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/models", nil)
		resp, _ := s.Do(req)

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
		}
	})

	t.Run("works with API key", func(t *testing.T) {
		req := s.RequestWithAPIKey("GET", "/v1/models", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 with API key, got %d", resp.StatusCode)
		}
		if result["object"] != "list" {
			t.Fatalf("expected object=list, got %v", result["object"])
		}
	})
}

// ============================================================================
// Billing E2E Tests
// ============================================================================

func TestBilling(t *testing.T) {
	s := NewSuite(t)

	t.Run("get balance", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/billing/balance", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		balance := result["balance"].(float64)
		if balance != 100.0 {
			t.Fatalf("expected balance 100.0, got %f", balance)
		}
	})

	t.Run("top up", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/billing/top-up", map[string]any{
			"amount":      50.0,
			"description": "E2E top up",
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		balance := result["balance"].(map[string]any)
		if balance["balance"].(float64) != 150.0 {
			t.Fatalf("expected balance 150.0 after top-up, got %f", balance["balance"])
		}
	})

	t.Run("list transactions", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/billing/transactions", nil)
		resp, body := s.Do(req)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}

		var result map[string]any
		json.Unmarshal(body, &result)

		transactions := result["transactions"].([]any)
		if len(transactions) < 2 {
			t.Fatalf("expected at least 2 transactions (initial + top-up), got %d", len(transactions))
		}
	})
}

// ============================================================================
// Usage E2E Tests
// ============================================================================

func TestUsage(t *testing.T) {
	s := NewSuite(t)

	t.Run("get usage summary with no data", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/usage/summary", nil)
		resp, body := s.Do(req)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}
	})

	t.Run("list usage logs with no data", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/usage/logs", nil)
		resp, body := s.Do(req)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, body)
		}
	})
}

// ============================================================================
// Rate Limit Config E2E Tests
// ============================================================================

func TestRateLimitConfig(t *testing.T) {
	s := NewSuite(t)

	tenantID := s.TenantID.String()

	t.Run("get default config", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/rate-limits/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		// Should return defaults
		if result["rpm"].(float64) != 60 {
			t.Fatalf("expected default rpm=60, got %v", result["rpm"])
		}
		if result["max_concurrent"].(float64) != 10 {
			t.Fatalf("expected default max_concurrent=10, got %v", result["max_concurrent"])
		}
	})

	t.Run("upsert custom config", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/rate-limits/"+tenantID, map[string]any{
			"rpm":            120,
			"max_concurrent": 20,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if result["rpm"].(float64) != 120 {
			t.Fatalf("expected rpm=120, got %v", result["rpm"])
		}
		if result["max_concurrent"].(float64) != 20 {
			t.Fatalf("expected max_concurrent=20, got %v", result["max_concurrent"])
		}
	})

	t.Run("get custom config", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/rate-limits/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if result["rpm"].(float64) != 120 {
			t.Fatalf("expected rpm=120 after upsert, got %v", result["rpm"])
		}
	})

	t.Run("delete config reverts to defaults", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/rate-limits/"+tenantID, nil)
		resp, _ := s.Do(req)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		// Get should return defaults again
		getReq := s.Request("GET", "/api/v1/rate-limits/"+tenantID, nil)
		var result map[string]any
		getResp := s.DoJSON(getReq, &result)

		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", getResp.StatusCode)
		}

		if result["rpm"].(float64) != 60 {
			t.Fatalf("expected default rpm=60 after delete, got %v", result["rpm"])
		}
	})
}

// ============================================================================
// Provider Key Test Endpoint E2E Tests
// ============================================================================

func TestProviderKeyTestEndpoint(t *testing.T) {
	s := NewSuite(t)

	t.Run("test key against mock upstream", func(t *testing.T) {
		// First, find the existing provider key
		req := s.Request("GET", "/api/v1/provider-keys/by-tenant/"+s.TenantID.String(), nil)
		resp, body := s.Do(req)

		// It might be empty for BYOK (our seeded key is managed, not tenant-owned)
		// Let's create a BYOK key pointing to the mock upstream
		mockURL := s.MockUpstream.URL
		createReq := s.Request("POST", "/api/v1/provider-keys", map[string]any{
			"provider_id": "openai",
			"tenant_id":   s.TenantID.String(),
			"token":       "sk-test-key-for-e2e",
			"base_url":    mockURL,
			"name":        "Test BYOK Key",
			"description": "Key for testing the test endpoint",
		})
		var created map[string]any
		resp = s.DoJSON(createReq, &created)

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d, body: %s", resp.StatusCode, body)
		}

		keyID := created["id"].(string)

		// Test the key
		testReq := s.Request("POST", "/api/v1/provider-keys/"+keyID+"/test", nil)
		var result map[string]any
		resp = s.DoJSON(testReq, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if result["valid"] != true {
			t.Fatalf("expected valid=true for mock upstream, got %v, message: %v", result["valid"], result["message"])
		}
		if result["latency_ms"] == nil {
			t.Fatal("expected latency_ms to be set")
		}
	})
}

// ============================================================================
// Model Fallback E2E Tests
// ============================================================================

func TestModelFallback(t *testing.T) {
	s := NewSuite(t)

	t.Run("create and list fallbacks", func(t *testing.T) {
		// Get two models from the seeded data
		modelsReq := s.Request("GET", "/api/v1/models", nil)
		var modelsResult map[string]any
		resp := s.DoJSON(modelsReq, &modelsResult)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		models := modelsResult["models"].([]any)
		if len(models) < 2 {
			t.Skip("need at least 2 models for fallback test")
		}

		modelA := models[0].(map[string]any)["id"].(string)
		modelB := models[1].(map[string]any)["id"].(string)

		// Create a fallback: modelA -> modelB
		createReq := s.Request("POST", "/api/v1/model-fallbacks", map[string]any{
			"model_id":          modelA,
			"fallback_model_id": modelB,
			"priority":          0,
		})
		var created map[string]any
		resp = s.DoJSON(createReq, &created)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		if created["model_id"] != modelA {
			t.Fatalf("expected model_id=%s, got %v", modelA, created["model_id"])
		}
		if created["fallback_model_id"] != modelB {
			t.Fatalf("expected fallback_model_id=%s, got %v", modelB, created["fallback_model_id"])
		}

		// List fallbacks for modelA
		listReq := s.Request("GET", "/api/v1/model-fallbacks/by-model/"+modelA, nil)
		var listResult map[string]any
		resp = s.DoJSON(listReq, &listResult)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		fallbacks := listResult["fallbacks"].([]any)
		if len(fallbacks) != 1 {
			t.Fatalf("expected 1 fallback, got %d", len(fallbacks))
		}

		// Delete the fallback
		fbID := created["id"].(string)
		delReq := s.Request("DELETE", "/api/v1/model-fallbacks/"+fbID, nil)
		resp, _ = s.Do(delReq)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		// Verify deleted
		listReq2 := s.Request("GET", "/api/v1/model-fallbacks/by-model/"+modelA, nil)
		var listResult2 map[string]any
		resp = s.DoJSON(listReq2, &listResult2)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		fallbacks2 := listResult2["fallbacks"].([]any)
		if len(fallbacks2) != 0 {
			t.Fatalf("expected 0 fallbacks after delete, got %d", len(fallbacks2))
		}
	})
}

// ============================================================================
// Spending Limits E2E Tests
// ============================================================================

func TestSpendingLimits(t *testing.T) {
	s := NewSuite(t)
	tenantID := s.TenantID.String()

	t.Run("get default (no limits)", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/spending-limits/"+tenantID, nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["message"] != "no spending limits configured" {
			t.Fatalf("expected 'no spending limits configured', got %v", result["message"])
		}
	})

	t.Run("upsert spending limits", func(t *testing.T) {
		req := s.Request("PUT", "/api/v1/spending-limits/"+tenantID, map[string]any{
			"daily_limit_usd":   10.0,
			"monthly_limit_usd": 100.0,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["daily_limit_usd"].(float64) != 10.0 {
			t.Fatalf("expected daily_limit=10, got %v", result["daily_limit_usd"])
		}
		if result["monthly_limit_usd"].(float64) != 100.0 {
			t.Fatalf("expected monthly_limit=100, got %v", result["monthly_limit_usd"])
		}
	})

	t.Run("check spending limit (should pass with 0 spend)", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/spending-limits/"+tenantID+"/check", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["allowed"] != true {
			t.Fatalf("expected allowed=true with 0 spend, got %v", result["allowed"])
		}
	})

	t.Run("delete spending limits", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/spending-limits/"+tenantID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

// ============================================================================
// Cache Invalidation E2E Tests
// ============================================================================

func TestCacheInvalidation(t *testing.T) {
	s := NewSuite(t)
	tenantID := s.TenantID.String()

	t.Run("invalidate cache for tenant", func(t *testing.T) {
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

	t.Run("invalidate all cache", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/cache/", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["keys_deleted"] == nil {
			t.Fatal("expected keys_deleted field")
		}
	})
}

// ============================================================================
// Cost Estimation E2E Tests
// ============================================================================

func TestCostEstimation(t *testing.T) {
	s := NewSuite(t)

	t.Run("estimate cost for known model", func(t *testing.T) {
		req := s.Request("POST", "/v1/cost/estimate", map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]any{
				{"role": "user", "content": "Hello, how are you?"},
			},
			"max_tokens": 1000,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		if result["model"] != "gpt-4o" {
			t.Fatalf("expected model=gpt-4o, got %v", result["model"])
		}
		if result["estimated_input_tokens"] == nil {
			t.Fatal("expected estimated_input_tokens")
		}
		if result["max_output_tokens"].(float64) != 1000 {
			t.Fatalf("expected max_output_tokens=1000, got %v", result["max_output_tokens"])
		}
		if result["estimated_total_cost_usd"] == nil {
			t.Fatal("expected estimated_total_cost_usd")
		}
		if result["provider"] == nil {
			t.Fatal("expected provider")
		}
	})

	t.Run("estimate cost for unknown model", func(t *testing.T) {
		req := s.Request("POST", "/v1/cost/estimate", map[string]any{
			"model":    "nonexistent-model",
			"messages": []map[string]any{},
		})
		resp, _ := s.Do(req)
		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for unknown model")
		}
	})

	t.Run("estimate cost defaults max_tokens", func(t *testing.T) {
		req := s.Request("POST", "/v1/cost/estimate", map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]any{
				{"role": "user", "content": "Hello"},
			},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["max_output_tokens"].(float64) == 0 {
			t.Fatal("expected non-zero max_output_tokens even without explicit max_tokens")
		}
	})
}

// ============================================================================
// Webhook E2E Tests
// ============================================================================

func TestWebhooks(t *testing.T) {
	s := NewSuite(t)

	var webhookID string

	t.Run("list available events", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks/events", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		events := result["events"].([]any)
		if len(events) == 0 {
			t.Fatal("expected at least one event type")
		}
	})

	t.Run("create webhook", func(t *testing.T) {
		req := s.Request("POST", "/api/v1/webhooks", map[string]any{
			"url":    "https://example.com/webhook",
			"events": []string{"request.completed", "request.failed"},
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}

		webhookID = result["id"].(string)
		if result["secret"] == nil || result["secret"] == "" {
			t.Fatal("expected secret to be returned on create")
		}
		if result["url"] != "https://example.com/webhook" {
			t.Fatalf("expected url, got %v", result["url"])
		}
		events := result["events"].([]any)
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
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

	t.Run("get webhook", func(t *testing.T) {
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
		enabled := false
		_ = enabled
		req := s.Request("PUT", "/api/v1/webhooks/"+webhookID, map[string]any{
			"enabled": false,
		})
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["enabled"] != false {
			t.Fatal("expected enabled=false after update")
		}
	})

	t.Run("list deliveries", func(t *testing.T) {
		req := s.Request("GET", "/api/v1/webhooks/"+webhookID+"/deliveries", nil)
		var result map[string]any
		resp := s.DoJSON(req, &result)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		deliveries := result["deliveries"].([]any)
		if len(deliveries) != 0 {
			t.Fatalf("expected 0 deliveries, got %d", len(deliveries))
		}
	})

	t.Run("delete webhook", func(t *testing.T) {
		req := s.Request("DELETE", "/api/v1/webhooks/"+webhookID, nil)
		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		// Verify deleted
		getReq := s.Request("GET", "/api/v1/webhooks", nil)
		var result map[string]any
		getResp := s.DoJSON(getReq, &result)
		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", getResp.StatusCode)
		}
		webhooks := result["webhooks"].([]any)
		if len(webhooks) != 0 {
			t.Fatalf("expected 0 webhooks after delete, got %d", len(webhooks))
		}
	})
}
