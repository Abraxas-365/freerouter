package gatewayapi_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// mockProvider is a test HTTP server that mimics upstream LLM providers.
// It captures requests and returns configurable responses.
type mockProvider struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []capturedRequest
	handler  func(w http.ResponseWriter, r *http.Request)
}

type capturedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

func newMockProvider() *mockProvider {
	mp := &mockProvider{}
	mp.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request
		body := make([]byte, 0)
		if r.Body != nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)
			body = buf.Bytes()
		}

		mp.mu.Lock()
		mp.requests = append(mp.requests, capturedRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
			Body:    body,
		})
		handler := mp.handler
		mp.mu.Unlock()

		if handler != nil {
			handler(w, r)
			return
		}

		// Default: return a standard OpenAI chat completion response
		mp.respondChatCompletion(w)
	}))
	return mp
}

func (mp *mockProvider) URL() string {
	return mp.server.URL
}

func (mp *mockProvider) Close() {
	mp.server.Close()
}

func (mp *mockProvider) Reset() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.requests = nil
	mp.handler = nil
}

func (mp *mockProvider) SetHandler(h func(w http.ResponseWriter, r *http.Request)) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.handler = h
}

func (mp *mockProvider) GetRequests() []capturedRequest {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	copied := make([]capturedRequest, len(mp.requests))
	copy(copied, mp.requests)
	return copied
}

func (mp *mockProvider) LastRequest() *capturedRequest {
	reqs := mp.GetRequests()
	if len(reqs) == 0 {
		return nil
	}
	return &reqs[len(reqs)-1]
}

// ============================================================================
// Standard responses
// ============================================================================

func (mp *mockProvider) respondChatCompletion(w http.ResponseWriter) {
	stop := "stop"
	resp := chatCompletionResp{
		ID:      "chatcmpl-test-123",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4o-2024-08-06",
		Choices: []chatCompletionChoice{{
			Index: 0,
			Message: &chatCompletionMessage{
				Role:    "assistant",
				Content: "Hello! How can I help you today?",
			},
			FinishReason: &stop,
		}},
		Usage: &chatCompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mp *mockProvider) respondChatCompletionStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	chunks := []string{
		`{"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
	}

	for _, chunk := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (mp *mockProvider) respondAnthropicNative(w http.ResponseWriter) {
	resp := map[string]any{
		"id":   "msg_test_123",
		"type": "message",
		"role": "assistant",
		"content": []map[string]string{
			{"type": "text", "text": "Hello from Claude!"},
		},
		"model":       "claude-sonnet-4-20250514",
		"stop_reason": "end_turn",
		"usage": map[string]int{
			"input_tokens":  12,
			"output_tokens": 6,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mp *mockProvider) respondAnthropicStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	events := []struct {
		event string
		data  string
	}{
		{"message_start", `{"type":"message_start","message":{"id":"msg_test_123","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"usage":{"input_tokens":12,"output_tokens":0}}}`},
		{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`},
		{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" from Claude!"}}`},
		{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":6}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}

	for _, evt := range events {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.event, evt.data)
		flusher.Flush()
	}
}

func (mp *mockProvider) respondToolCall(w http.ResponseWriter) {
	stop := "tool_calls"
	resp := chatCompletionResp{
		ID:      "chatcmpl-test-tool",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4o",
		Choices: []chatCompletionChoice{{
			Index: 0,
			Message: &chatCompletionMessage{
				Role: "assistant",
				ToolCalls: []chatCompletionToolCall{{
					ID:   "call_abc123",
					Type: "function",
					Function: chatCompletionToolCallFn{
						Name:      "get_weather",
						Arguments: `{"city":"NYC"}`,
					},
				}},
			},
			FinishReason: &stop,
		}},
		Usage: &chatCompletionUsage{
			PromptTokens:     15,
			CompletionTokens: 20,
			TotalTokens:      35,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (mp *mockProvider) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "server_error",
			"code":    status,
		},
	})
}

// ============================================================================
// SSE stream reader utility
// ============================================================================

type streamResult struct {
	Events     []streamEvent
	RawContent string
	HasDone    bool
}

type streamEvent struct {
	EventType string
	Data      json.RawMessage
}

func readSSEStream(body []byte) streamResult {
	var result streamResult
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				result.HasDone = true
				continue
			}

			result.Events = append(result.Events, streamEvent{
				EventType: currentEvent,
				Data:      json.RawMessage(data),
			})
			currentEvent = ""
		}
	}

	return result
}

// ============================================================================
// Response types for mock server (avoid importing gateway package in test helpers)
// ============================================================================

type chatCompletionResp struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []chatCompletionChoice `json:"choices"`
	Usage   *chatCompletionUsage   `json:"usage,omitempty"`
}

type chatCompletionChoice struct {
	Index        int                    `json:"index"`
	Message      *chatCompletionMessage `json:"message,omitempty"`
	FinishReason *string                `json:"finish_reason,omitempty"`
}

type chatCompletionMessage struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content,omitempty"`
	ToolCalls []chatCompletionToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function chatCompletionToolCallFn `json:"function"`
}

type chatCompletionToolCallFn struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
