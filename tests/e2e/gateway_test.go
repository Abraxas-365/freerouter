package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ============================================================================
// POST /v1/chat/completions E2E Tests
// ============================================================================

func TestChatCompletions(t *testing.T) {
	s := NewSuite(t)

	t.Run("non-streaming basic request", func(t *testing.T) {
		req := s.Request("POST", "/v1/chat/completions", map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		})

		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "chat.completion" {
			t.Fatalf("expected object=chat.completion, got %v", result["object"])
		}

		choices := result["choices"].([]any)
		if len(choices) == 0 {
			t.Fatal("expected choices in response")
		}

		choice := choices[0].(map[string]any)
		msg := choice["message"].(map[string]any)
		if msg["role"] != "assistant" {
			t.Fatalf("expected role=assistant, got %v", msg["role"])
		}
		if msg["content"] == nil || msg["content"] == "" {
			t.Fatal("expected content in response")
		}

		// Verify usage
		usage := result["usage"].(map[string]any)
		if usage["total_tokens"].(float64) == 0 {
			t.Fatal("expected non-zero total_tokens")
		}
	})

	t.Run("streaming request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model":  "gpt-4o",
			"stream": true,
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
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

		// Parse SSE stream
		scanner := bufio.NewScanner(resp.Body)
		var chunks []map[string]any
		hasDone := false

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					hasDone = true
					continue
				}
				var chunk map[string]any
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					chunks = append(chunks, chunk)
				}
			}
		}

		if len(chunks) == 0 {
			t.Fatal("expected stream chunks")
		}
		if !hasDone {
			t.Fatal("expected [DONE] marker")
		}

		// First chunk should have role
		firstChoice := chunks[0]["choices"].([]any)[0].(map[string]any)
		delta := firstChoice["delta"].(map[string]any)
		if delta["role"] != "assistant" {
			t.Fatalf("expected first chunk role=assistant, got %v", delta["role"])
		}
	})

	t.Run("missing model returns error", func(t *testing.T) {
		req := s.Request("POST", "/v1/chat/completions", map[string]any{
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		})

		resp, _ := s.Do(req)
		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for missing model")
		}
	})

	t.Run("invalid model returns error", func(t *testing.T) {
		req := s.Request("POST", "/v1/chat/completions", map[string]any{
			"model": "nonexistent-model",
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		})

		resp, _ := s.Do(req)
		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for invalid model")
		}
	})

	t.Run("requires auth", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model":    "gpt-4o",
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
		})
		req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := s.Do(req)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
		}
	})

	t.Run("works with API key", func(t *testing.T) {
		req := s.RequestWithAPIKey("POST", "/v1/chat/completions", map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
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
}

// ============================================================================
// POST /v1/messages E2E Tests (Anthropic Messages API)
// ============================================================================

func TestAnthropicMessages(t *testing.T) {
	s := NewSuite(t)

	t.Run("non-streaming basic request", func(t *testing.T) {
		req := s.Request("POST", "/v1/messages", map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 1024,
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		})

		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["type"] != "message" {
			t.Fatalf("expected type=message, got %v", result["type"])
		}
		if result["role"] != "assistant" {
			t.Fatalf("expected role=assistant, got %v", result["role"])
		}

		// Content should be an array of blocks
		content := result["content"].([]any)
		if len(content) == 0 {
			t.Fatal("expected content blocks")
		}
		block := content[0].(map[string]any)
		if block["type"] != "text" {
			t.Fatalf("expected text block, got %v", block["type"])
		}
		if block["text"] == nil || block["text"] == "" {
			t.Fatal("expected text content")
		}

		// stop_reason should be Anthropic format
		stopReason := result["stop_reason"].(string)
		if stopReason != "end_turn" {
			t.Fatalf("expected stop_reason=end_turn, got %s", stopReason)
		}

		// Usage should be Anthropic format
		usage := result["usage"].(map[string]any)
		if usage["input_tokens"].(float64) == 0 {
			t.Fatal("expected non-zero input_tokens")
		}
	})

	t.Run("with system message", func(t *testing.T) {
		req := s.Request("POST", "/v1/messages", map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 1024,
			"system":     "You are a helpful assistant.",
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
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

	t.Run("streaming", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model":      "gpt-4o",
			"max_tokens": 1024,
			"stream":     true,
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		})

		req, _ := http.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
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

		// Parse Anthropic SSE events
		scanner := bufio.NewScanner(resp.Body)
		eventTypes := make(map[string]bool)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				eventType := strings.TrimPrefix(line, "event: ")
				eventTypes[eventType] = true
			}
		}

		// Verify expected Anthropic event types
		expectedEvents := []string{"message_start", "content_block_start", "content_block_stop", "message_delta", "message_stop"}
		for _, evt := range expectedEvents {
			if !eventTypes[evt] {
				t.Errorf("expected event type %s in stream", evt)
			}
		}
	})

	t.Run("missing max_tokens returns error", func(t *testing.T) {
		req := s.Request("POST", "/v1/messages", map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]any{
				{"role": "user", "content": "hello"},
			},
		})

		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode == http.StatusOK {
			t.Fatal("expected error for missing max_tokens")
		}
		if result["type"] != "error" {
			t.Fatalf("expected Anthropic error format, got %v", result["type"])
		}
	})
}

// ============================================================================
// POST /v1/responses E2E Tests (OpenAI Responses API)
// ============================================================================

func TestResponsesAPI(t *testing.T) {
	s := NewSuite(t)

	t.Run("non-streaming string input", func(t *testing.T) {
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
			t.Fatalf("expected object=response, got %v", result["object"])
		}
		if result["status"] != "completed" {
			t.Fatalf("expected status=completed, got %v", result["status"])
		}

		output := result["output"].([]any)
		if len(output) == 0 {
			t.Fatal("expected output items")
		}
		msg := output[0].(map[string]any)
		if msg["type"] != "message" {
			t.Fatalf("expected type=message, got %v", msg["type"])
		}

		content := msg["content"].([]any)
		if len(content) == 0 {
			t.Fatal("expected content in message")
		}
		textBlock := content[0].(map[string]any)
		if textBlock["type"] != "output_text" {
			t.Fatalf("expected output_text, got %v", textBlock["type"])
		}
	})

	t.Run("with instructions", func(t *testing.T) {
		req := s.Request("POST", "/v1/responses", map[string]any{
			"model":        "gpt-4o",
			"instructions": "You are a pirate.",
			"input":        "hello",
		})

		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if result["object"] != "response" {
			t.Fatalf("expected response object, got %v", result["object"])
		}
	})

	t.Run("with message input items", func(t *testing.T) {
		req := s.Request("POST", "/v1/responses", map[string]any{
			"model": "gpt-4o",
			"input": []map[string]any{
				{"type": "message", "role": "user", "content": "hello"},
				{"type": "message", "role": "assistant", "content": "hi"},
				{"type": "message", "role": "user", "content": "how are you?"},
			},
		})

		var result map[string]any
		resp := s.DoJSON(req, &result)

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("streaming", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"model":  "gpt-4o",
			"input":  "hello",
			"stream": true,
		})

		req, _ := http.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
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

		// Parse Responses API SSE events
		scanner := bufio.NewScanner(resp.Body)
		eventTypes := make(map[string]bool)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				eventType := strings.TrimPrefix(line, "event: ")
				eventTypes[eventType] = true
			}
		}

		// Verify expected Responses API event types
		expectedEvents := []string{"response.created", "response.output_item.added", "response.content_part.added"}
		for _, evt := range expectedEvents {
			if !eventTypes[evt] {
				t.Errorf("expected event type %s in stream", evt)
			}
		}

		// Should have either completed or failed
		if !eventTypes["response.completed"] && !eventTypes["response.failed"] {
			t.Error("expected response.completed or response.failed")
		}
	})
}

// ============================================================================
// Cross-cutting: Billing integration with gateway
// ============================================================================

func TestBillingDebitAfterRequest(t *testing.T) {
	s := NewSuite(t)

	// Get initial balance
	req := s.Request("GET", "/api/v1/billing/balance", nil)
	var balanceBefore map[string]any
	s.DoJSON(req, &balanceBefore)
	initialBalance := balanceBefore["balance"].(float64)

	// Make a chat completion request
	req = s.Request("POST", "/v1/chat/completions", map[string]any{
		"model": "gpt-4o",
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	})
	resp, _ := s.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Give async operations a moment
	// (billing debit is synchronous, but let's be safe)

	// Get balance after
	req = s.Request("GET", "/api/v1/billing/balance", nil)
	var balanceAfter map[string]any
	s.DoJSON(req, &balanceAfter)
	finalBalance := balanceAfter["balance"].(float64)

	if finalBalance >= initialBalance {
		t.Fatalf("expected balance to decrease after request: before=%f, after=%f", initialBalance, finalBalance)
	}

	t.Logf("Balance decreased: $%.6f -> $%.6f (cost: $%.6f)", initialBalance, finalBalance, initialBalance-finalBalance)
}
