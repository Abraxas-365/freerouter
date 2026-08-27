package gateway

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ============================================================================
// OpenAI Translator (passthrough)
// ============================================================================

func TestOpenAITranslator_TransformRequest_Passthrough(t *testing.T) {
	tr := &OpenAITranslator{}
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)

	out, err := tr.TransformRequest(body, "gpt-4o")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(body) {
		t.Fatalf("expected passthrough, got %s", out)
	}
}

func TestOpenAITranslator_TransformResponse(t *testing.T) {
	tr := &OpenAITranslator{}
	body := []byte(`{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"model": "gpt-4o",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "hi"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`)

	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "chatcmpl-123" {
		t.Fatalf("expected id chatcmpl-123, got %s", resp.ID)
	}
	if resp.Usage.TotalTokens != 7 {
		t.Fatalf("expected 7 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestOpenAITranslator_TransformStreamEvent_Done(t *testing.T) {
	tr := &OpenAITranslator{}

	_, done, err := tr.TransformStreamEvent([]byte("[DONE]"))
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("expected done=true for [DONE]")
	}
}

func TestOpenAITranslator_TransformStreamEvent_Chunk(t *testing.T) {
	tr := &OpenAITranslator{}
	chunk := []byte(`{"id":"chatcmpl-123","choices":[{"delta":{"content":"hi"}}]}`)

	out, done, err := tr.TransformStreamEvent(chunk)
	if err != nil {
		t.Fatal(err)
	}
	if done {
		t.Fatal("expected done=false")
	}
	if string(out) != string(chunk) {
		t.Fatalf("expected passthrough, got %s", out)
	}
}

// ============================================================================
// Anthropic Translator
// ============================================================================

func TestAnthropicTranslator_TransformRequest_Basic(t *testing.T) {
	tr := &AnthropicTranslator{}
	body := []byte(`{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "hello"}
		],
		"temperature": 0.7,
		"max_tokens": 1024
	}`)

	out, err := tr.TransformRequest(body, "claude-sonnet-4-20250514")
	if err != nil {
		t.Fatal(err)
	}

	var ar map[string]any
	if err := json.Unmarshal(out, &ar); err != nil {
		t.Fatal(err)
	}

	// System should be top-level, not in messages
	if ar["system"] != "You are helpful." {
		t.Fatalf("expected system to be extracted, got %v", ar["system"])
	}

	// Messages should only have the user message
	msgs := ar["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (system extracted), got %d", len(msgs))
	}
	msg := msgs[0].(map[string]any)
	if msg["role"] != "user" {
		t.Fatalf("expected user role, got %s", msg["role"])
	}

	// max_tokens must be present
	if ar["max_tokens"].(float64) != 1024 {
		t.Fatalf("expected max_tokens=1024, got %v", ar["max_tokens"])
	}

	// Model should be the external ID
	if ar["model"] != "claude-sonnet-4-20250514" {
		t.Fatalf("expected model claude-sonnet-4-20250514, got %v", ar["model"])
	}
}

func TestAnthropicTranslator_TransformRequest_DefaultMaxTokens(t *testing.T) {
	tr := &AnthropicTranslator{}
	body := []byte(`{"model": "claude-sonnet-4-20250514", "messages": [{"role": "user", "content": "hi"}]}`)

	out, err := tr.TransformRequest(body, "claude-sonnet-4-20250514")
	if err != nil {
		t.Fatal(err)
	}

	var ar map[string]any
	if err := json.Unmarshal(out, &ar); err != nil {
		t.Fatal(err)
	}

	if ar["max_tokens"].(float64) != 4096 {
		t.Fatalf("expected default max_tokens=4096, got %v", ar["max_tokens"])
	}
}

func TestAnthropicTranslator_TransformRequest_ToolCalls(t *testing.T) {
	tr := &AnthropicTranslator{}
	body := []byte(`{
		"model": "claude-sonnet-4-20250514",
		"messages": [{"role": "user", "content": "what's the weather?"}],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get weather for a city",
				"parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
			}
		}],
		"tool_choice": "auto"
	}`)

	out, err := tr.TransformRequest(body, "claude-sonnet-4-20250514")
	if err != nil {
		t.Fatal(err)
	}

	var ar map[string]any
	if err := json.Unmarshal(out, &ar); err != nil {
		t.Fatal(err)
	}

	// Tools should be converted to Anthropic format
	tools := ar["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "get_weather" {
		t.Fatalf("expected tool name get_weather, got %v", tool["name"])
	}
	// Anthropic uses input_schema instead of parameters
	if tool["input_schema"] == nil {
		t.Fatal("expected input_schema to be set")
	}

	// tool_choice "auto" → {"type": "auto"}
	tc := ar["tool_choice"].(map[string]any)
	if tc["type"] != "auto" {
		t.Fatalf("expected tool_choice type auto, got %v", tc["type"])
	}
}

func TestAnthropicTranslator_TransformRequest_ToolResult(t *testing.T) {
	tr := &AnthropicTranslator{}
	body := []byte(`{
		"model": "claude-sonnet-4-20250514",
		"messages": [
			{"role": "user", "content": "what's the weather?"},
			{"role": "assistant", "content": "", "tool_calls": [{"id": "tc_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"NYC\"}"}}]},
			{"role": "tool", "tool_call_id": "tc_1", "content": "72°F and sunny"}
		]
	}`)

	out, err := tr.TransformRequest(body, "claude-sonnet-4-20250514")
	if err != nil {
		t.Fatal(err)
	}

	var ar map[string]any
	if err := json.Unmarshal(out, &ar); err != nil {
		t.Fatal(err)
	}

	msgs := ar["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// Assistant message should have tool_use content blocks
	assistantMsg := msgs[1].(map[string]any)
	assistantContent := assistantMsg["content"].([]any)
	toolUseBlock := assistantContent[0].(map[string]any)
	if toolUseBlock["type"] != "tool_use" {
		t.Fatalf("expected tool_use block, got %v", toolUseBlock["type"])
	}

	// Tool result: role should be "user" with tool_result content block
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["role"] != "user" {
		t.Fatalf("expected tool result role=user, got %v", toolMsg["role"])
	}
	toolContent := toolMsg["content"].([]any)
	toolResult := toolContent[0].(map[string]any)
	if toolResult["type"] != "tool_result" {
		t.Fatalf("expected tool_result block, got %v", toolResult["type"])
	}
	if toolResult["tool_use_id"] != "tc_1" {
		t.Fatalf("expected tool_use_id=tc_1, got %v", toolResult["tool_use_id"])
	}
}

func TestAnthropicTranslator_TransformResponse(t *testing.T) {
	tr := &AnthropicTranslator{}
	body := []byte(`{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"model": "claude-sonnet-4-20250514",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`)

	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.ID != "msg_123" {
		t.Fatalf("expected id msg_123, got %s", resp.ID)
	}
	if resp.Object != "chat.completion" {
		t.Fatalf("expected object chat.completion, got %s", resp.Object)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}

	msg := resp.Choices[0].Message
	if msg.Role != "assistant" {
		t.Fatalf("expected role assistant, got %s", msg.Role)
	}
	if msg.Content != "Hello!" {
		t.Fatalf("expected content Hello!, got %v", msg.Content)
	}

	// stop_reason "end_turn" → finish_reason "stop"
	if *resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish_reason stop, got %s", *resp.Choices[0].FinishReason)
	}

	if resp.Usage.PromptTokens != 10 {
		t.Fatalf("expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 5 {
		t.Fatalf("expected 5 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
}

func TestAnthropicTranslator_TransformResponse_ToolUse(t *testing.T) {
	tr := &AnthropicTranslator{}
	body := []byte(`{
		"id": "msg_456",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Let me check."},
			{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": {"city": "NYC"}}
		],
		"model": "claude-sonnet-4-20250514",
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 20, "output_tokens": 15}
	}`)

	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}

	msg := resp.Choices[0].Message
	if msg.Content != "Let me check." {
		t.Fatalf("expected text content, got %v", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("expected tool call id toolu_1, got %s", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("expected function name get_weather, got %s", msg.ToolCalls[0].Function.Name)
	}

	// stop_reason "tool_use" → finish_reason "tool_calls"
	if *resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason tool_calls, got %s", *resp.Choices[0].FinishReason)
	}
}

func TestAnthropicTranslator_TransformStreamEvent(t *testing.T) {
	tr := &AnthropicTranslator{}

	tests := []struct {
		name       string
		input      string
		wantDone   bool
		wantNil    bool
		checkChunk func(t *testing.T, chunk ChatStreamChunk)
	}{
		{
			name:  "message_start",
			input: `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-20250514","role":"assistant"}}`,
			checkChunk: func(t *testing.T, chunk ChatStreamChunk) {
				if chunk.ID != "msg_1" {
					t.Fatalf("expected id msg_1, got %s", chunk.ID)
				}
				if chunk.Choices[0].Delta.Role != "assistant" {
					t.Fatalf("expected role assistant, got %s", chunk.Choices[0].Delta.Role)
				}
			},
		},
		{
			name:  "content_block_delta text",
			input: `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			checkChunk: func(t *testing.T, chunk ChatStreamChunk) {
				if chunk.Choices[0].Delta.Content != "Hello" {
					t.Fatalf("expected content Hello, got %v", chunk.Choices[0].Delta.Content)
				}
			},
		},
		{
			name:  "message_delta with stop",
			input: `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`,
			checkChunk: func(t *testing.T, chunk ChatStreamChunk) {
				if *chunk.Choices[0].FinishReason != "stop" {
					t.Fatalf("expected finish_reason stop, got %s", *chunk.Choices[0].FinishReason)
				}
				if chunk.Usage.CompletionTokens != 10 {
					t.Fatalf("expected 10 completion tokens, got %d", chunk.Usage.CompletionTokens)
				}
			},
		},
		{
			name:     "message_stop",
			input:    `{"type":"message_stop"}`,
			wantDone: true,
		},
		{
			name:    "ping",
			input:   `{"type":"ping"}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, done, err := tr.TransformStreamEvent([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if done != tt.wantDone {
				t.Fatalf("expected done=%v, got %v", tt.wantDone, done)
			}
			if tt.wantNil {
				if out != nil {
					t.Fatalf("expected nil output, got %s", out)
				}
				return
			}
			if tt.wantDone {
				return
			}
			var chunk ChatStreamChunk
			if err := json.Unmarshal(out, &chunk); err != nil {
				t.Fatalf("failed to parse chunk: %v", err)
			}
			if tt.checkChunk != nil {
				tt.checkChunk(t, chunk)
			}
		})
	}
}

func TestAnthropicTranslator_StopReasonMapping(t *testing.T) {
	tests := []struct {
		anthropic string
		openai    string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"stop_sequence", "stop"},
	}

	for _, tt := range tests {
		t.Run(tt.anthropic, func(t *testing.T) {
			reason := tt.anthropic
			mapped := mapAnthropicStopReason(&reason)
			if *mapped != tt.openai {
				t.Fatalf("expected %s, got %s", tt.openai, *mapped)
			}
		})
	}
}

// ============================================================================
// Google Translator
// ============================================================================

func TestGoogleTranslator_TransformRequest_Basic(t *testing.T) {
	tr := &GoogleTranslator{}
	body := []byte(`{
		"model": "gemini-2.0-flash",
		"messages": [
			{"role": "system", "content": "Be concise."},
			{"role": "user", "content": "hello"}
		],
		"temperature": 0.5,
		"max_tokens": 2048
	}`)

	out, err := tr.TransformRequest(body, "gemini-2.0-flash")
	if err != nil {
		t.Fatal(err)
	}

	var gr map[string]any
	if err := json.Unmarshal(out, &gr); err != nil {
		t.Fatal(err)
	}

	// System should be in systemInstruction, not contents
	si := gr["systemInstruction"].(map[string]any)
	parts := si["parts"].([]any)
	if parts[0].(map[string]any)["text"] != "Be concise." {
		t.Fatalf("expected system instruction, got %v", parts[0])
	}

	// Contents should only have the user message
	contents := gr["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("expected 1 content (system extracted), got %d", len(contents))
	}
	content := contents[0].(map[string]any)
	if content["role"] != "user" {
		t.Fatalf("expected user role, got %v", content["role"])
	}

	// generationConfig
	gc := gr["generationConfig"].(map[string]any)
	if gc["temperature"].(float64) != 0.5 {
		t.Fatalf("expected temperature 0.5, got %v", gc["temperature"])
	}
	if gc["maxOutputTokens"].(float64) != 2048 {
		t.Fatalf("expected maxOutputTokens 2048, got %v", gc["maxOutputTokens"])
	}
}

func TestGoogleTranslator_TransformRequest_AssistantRole(t *testing.T) {
	tr := &GoogleTranslator{}
	body := []byte(`{
		"model": "gemini-2.0-flash",
		"messages": [
			{"role": "user", "content": "hi"},
			{"role": "assistant", "content": "hello"},
			{"role": "user", "content": "how are you?"}
		]
	}`)

	out, err := tr.TransformRequest(body, "gemini-2.0-flash")
	if err != nil {
		t.Fatal(err)
	}

	var gr map[string]any
	if err := json.Unmarshal(out, &gr); err != nil {
		t.Fatal(err)
	}

	contents := gr["contents"].([]any)
	if len(contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(contents))
	}

	// assistant → model
	assistantContent := contents[1].(map[string]any)
	if assistantContent["role"] != "model" {
		t.Fatalf("expected model role for assistant, got %v", assistantContent["role"])
	}
}

func TestGoogleTranslator_TransformRequest_Tools(t *testing.T) {
	tr := &GoogleTranslator{}
	body := []byte(`{
		"model": "gemini-2.0-flash",
		"messages": [{"role": "user", "content": "weather?"}],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get weather",
				"parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
			}
		}]
	}`)

	out, err := tr.TransformRequest(body, "gemini-2.0-flash")
	if err != nil {
		t.Fatal(err)
	}

	var gr map[string]any
	if err := json.Unmarshal(out, &gr); err != nil {
		t.Fatal(err)
	}

	tools := gr["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool group, got %d", len(tools))
	}
	toolGroup := tools[0].(map[string]any)
	decls := toolGroup["functionDeclarations"].([]any)
	if len(decls) != 1 {
		t.Fatalf("expected 1 function declaration, got %d", len(decls))
	}
	if decls[0].(map[string]any)["name"] != "get_weather" {
		t.Fatalf("expected get_weather, got %v", decls[0].(map[string]any)["name"])
	}
}

func TestGoogleTranslator_TransformResponse(t *testing.T) {
	tr := &GoogleTranslator{}
	body := []byte(`{
		"candidates": [{
			"content": {
				"parts": [{"text": "Hello there!"}],
				"role": "model"
			},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {
			"promptTokenCount": 8,
			"candidatesTokenCount": 3,
			"totalTokenCount": 11
		},
		"modelVersion": "gemini-2.0-flash"
	}`)

	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Object != "chat.completion" {
		t.Fatalf("expected chat.completion, got %s", resp.Object)
	}
	if resp.Model != "gemini-2.0-flash" {
		t.Fatalf("expected gemini-2.0-flash, got %s", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}

	msg := resp.Choices[0].Message
	if msg.Content != "Hello there!" {
		t.Fatalf("expected Hello there!, got %v", msg.Content)
	}
	if *resp.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected stop, got %s", *resp.Choices[0].FinishReason)
	}

	if resp.Usage.PromptTokens != 8 {
		t.Fatalf("expected 8 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 3 {
		t.Fatalf("expected 3 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
}

func TestGoogleTranslator_TransformResponse_ToolCall(t *testing.T) {
	tr := &GoogleTranslator{}
	body := []byte(`{
		"candidates": [{
			"content": {
				"parts": [{"functionCall": {"name": "get_weather", "args": {"city": "NYC"}}}],
				"role": "model"
			},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5, "totalTokenCount": 15}
	}`)

	resp, err := tr.TransformResponse(body)
	if err != nil {
		t.Fatal(err)
	}

	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", msg.ToolCalls[0].Function.Name)
	}
}

func TestGoogleTranslator_TransformStreamEvent(t *testing.T) {
	tr := &GoogleTranslator{}

	// Regular content chunk
	data := []byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "Hi"}], "role": "model"},
			"index": 0
		}]
	}`)

	out, done, err := tr.TransformStreamEvent(data)
	if err != nil {
		t.Fatal(err)
	}
	if done {
		t.Fatal("expected done=false")
	}

	var chunk ChatStreamChunk
	if err := json.Unmarshal(out, &chunk); err != nil {
		t.Fatal(err)
	}
	if chunk.Object != "chat.completion.chunk" {
		t.Fatalf("expected chat.completion.chunk, got %s", chunk.Object)
	}
	if chunk.Choices[0].Delta.Content != "Hi" {
		t.Fatalf("expected content Hi, got %v", chunk.Choices[0].Delta.Content)
	}
}

func TestGoogleTranslator_TransformStreamEvent_Final(t *testing.T) {
	tr := &GoogleTranslator{}

	data := []byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "."}], "role": "model"},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 10, "totalTokenCount": 15}
	}`)

	out, done, err := tr.TransformStreamEvent(data)
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("expected done=true for STOP")
	}

	var chunk ChatStreamChunk
	if err := json.Unmarshal(out, &chunk); err != nil {
		t.Fatal(err)
	}
	if *chunk.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish_reason stop, got %s", *chunk.Choices[0].FinishReason)
	}
	if chunk.Usage.TotalTokens != 15 {
		t.Fatalf("expected 15 total tokens, got %d", chunk.Usage.TotalTokens)
	}
}

func TestGoogleTranslator_FinishReasonMapping(t *testing.T) {
	tests := []struct {
		google string
		openai string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
	}

	for _, tt := range tests {
		t.Run(tt.google, func(t *testing.T) {
			mapped := mapGeminiFinishReason(tt.google)
			if *mapped != tt.openai {
				t.Fatalf("expected %s, got %s", tt.openai, *mapped)
			}
		})
	}
}

// ============================================================================
// GetTranslator factory
// ============================================================================

func TestGetTranslator(t *testing.T) {
	tests := []struct {
		provider string
		wantType string
	}{
		{"openai", "*gateway.OpenAITranslator"},
		{"groq", "*gateway.OpenAITranslator"},
		{"together", "*gateway.OpenAITranslator"},
		{"mistral", "*gateway.OpenAITranslator"},
		{"deepseek", "*gateway.OpenAITranslator"},
		{"xai", "*gateway.OpenAITranslator"},
		{"anthropic", "*gateway.AnthropicTranslator"},
		{"google", "*gateway.GoogleTranslator"},
		{"google-ai-studio", "*gateway.GoogleTranslator"},
		{"google-vertex", "*gateway.GoogleTranslator"},
		{"unknown-provider", "*gateway.OpenAITranslator"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			tr := GetTranslator(tt.provider)
			got := fmt.Sprintf("%T", tr)
			if got != tt.wantType {
				t.Fatalf("expected %s, got %s", tt.wantType, got)
			}
		})
	}
}
