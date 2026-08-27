package gatewayapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
)

// ============================================================================
// anthropicToChatRequest conversion tests
// ============================================================================

func TestAnthropicToChatRequest_BasicMessage(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages: []anthropicMsg{
			{Role: "user", Content: "hello"},
		},
	}

	cr := anthropicToChatRequest(req)

	if cr.Model != "claude-sonnet-4-5" {
		t.Fatalf("expected model claude-sonnet-4-5, got %s", cr.Model)
	}
	if *cr.MaxTokens != 1024 {
		t.Fatalf("expected max_tokens 1024, got %d", *cr.MaxTokens)
	}
	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "user" {
		t.Fatalf("expected role user, got %s", cr.Messages[0].Role)
	}
	if cr.Messages[0].Content != "hello" {
		t.Fatalf("expected content hello, got %v", cr.Messages[0].Content)
	}
}

func TestAnthropicToChatRequest_SystemMessage(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		System:    "You are a helpful assistant.",
		Messages: []anthropicMsg{
			{Role: "user", Content: "hi"},
		},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "system" {
		t.Fatalf("expected first message role=system, got %s", cr.Messages[0].Role)
	}
	if cr.Messages[0].Content != "You are a helpful assistant." {
		t.Fatalf("expected system content, got %v", cr.Messages[0].Content)
	}
}

func TestAnthropicToChatRequest_SystemAsBlocks(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		System: []any{
			map[string]any{"type": "text", "text": "Part 1."},
			map[string]any{"type": "text", "text": "Part 2."},
		},
		Messages: []anthropicMsg{
			{Role: "user", Content: "hi"},
		},
	}

	cr := anthropicToChatRequest(req)

	if cr.Messages[0].Content != "Part 1.\nPart 2." {
		t.Fatalf("expected joined system content, got %v", cr.Messages[0].Content)
	}
}

func TestAnthropicToChatRequest_ToolUseContentBlocks(t *testing.T) {
	content := []any{
		map[string]any{
			"type":  "tool_use",
			"id":    "toolu_1",
			"name":  "get_weather",
			"input": map[string]any{"city": "NYC"},
		},
	}

	req := &anthropicMessagesRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages: []anthropicMsg{
			{Role: "assistant", Content: content},
		},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	msg := cr.Messages[0]
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "toolu_1" {
		t.Fatalf("expected tool call id toolu_1, got %s", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("expected function name get_weather, got %s", msg.ToolCalls[0].Function.Name)
	}
}

func TestAnthropicToChatRequest_ToolResultBlocks(t *testing.T) {
	content := []any{
		map[string]any{
			"type":        "tool_result",
			"tool_use_id": "toolu_1",
			"content":     "72°F and sunny",
		},
	}

	req := &anthropicMessagesRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages: []anthropicMsg{
			{Role: "user", Content: content},
		},
	}

	cr := anthropicToChatRequest(req)

	// Tool results should become role=tool messages
	found := false
	for _, msg := range cr.Messages {
		if msg.Role == "tool" {
			found = true
			if msg.ToolCallID != "toolu_1" {
				t.Fatalf("expected tool_call_id toolu_1, got %s", msg.ToolCallID)
			}
			if msg.Content != "72°F and sunny" {
				t.Fatalf("expected content, got %v", msg.Content)
			}
		}
	}
	if !found {
		t.Fatal("expected tool message, not found")
	}
}

func TestAnthropicToChatRequest_ToolDefinitions(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 1024,
		Messages:  []anthropicMsg{{Role: "user", Content: "weather?"}},
		Tools: []anthropicToolDef{
			{
				Name:        "get_weather",
				Description: "Get weather for a city",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		ToolChoice: map[string]any{"type": "auto"},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cr.Tools))
	}
	if cr.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", cr.Tools[0].Function.Name)
	}
	if cr.ToolChoice != "auto" {
		t.Fatalf("expected tool_choice auto, got %v", cr.ToolChoice)
	}
}

func TestAnthropicToChatRequest_ToolChoiceAny(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:      "claude-sonnet-4-5",
		MaxTokens:  1024,
		Messages:   []anthropicMsg{{Role: "user", Content: "hi"}},
		ToolChoice: map[string]any{"type": "any"},
	}

	cr := anthropicToChatRequest(req)

	if cr.ToolChoice != "required" {
		t.Fatalf("expected tool_choice required (mapped from any), got %v", cr.ToolChoice)
	}
}

func TestAnthropicToChatRequest_ToolChoiceSpecific(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:      "claude-sonnet-4-5",
		MaxTokens:  1024,
		Messages:   []anthropicMsg{{Role: "user", Content: "hi"}},
		ToolChoice: map[string]any{"type": "tool", "name": "get_weather"},
	}

	cr := anthropicToChatRequest(req)

	tc, ok := cr.ToolChoice.(map[string]any)
	if !ok {
		t.Fatalf("expected map tool_choice, got %T", cr.ToolChoice)
	}
	if tc["type"] != "function" {
		t.Fatalf("expected type function, got %v", tc["type"])
	}
	fn := tc["function"].(map[string]string)
	if fn["name"] != "get_weather" {
		t.Fatalf("expected name get_weather, got %s", fn["name"])
	}
}

func TestAnthropicToChatRequest_Temperature(t *testing.T) {
	temp := 0.7
	topP := 0.9
	req := &anthropicMessagesRequest{
		Model:       "claude-sonnet-4-5",
		MaxTokens:   1024,
		Temperature: &temp,
		TopP:        &topP,
		Messages:    []anthropicMsg{{Role: "user", Content: "hi"}},
	}

	cr := anthropicToChatRequest(req)

	if *cr.Temperature != 0.7 {
		t.Fatalf("expected temperature 0.7, got %f", *cr.Temperature)
	}
	if *cr.TopP != 0.9 {
		t.Fatalf("expected top_p 0.9, got %f", *cr.TopP)
	}
}

// ============================================================================
// chatResponseToAnthropic conversion tests
// ============================================================================

func TestChatResponseToAnthropic_Basic(t *testing.T) {
	stop := "stop"
	resp := &gateway.ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []gateway.Choice{{
			Index:        0,
			Message:      &gateway.Message{Role: "assistant", Content: "Hello!"},
			FinishReason: &stop,
		}},
		Usage: &gateway.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	ar := chatResponseToAnthropic(resp, "claude-sonnet-4-5")

	if ar.Type != "message" {
		t.Fatalf("expected type message, got %s", ar.Type)
	}
	if ar.Role != "assistant" {
		t.Fatalf("expected role assistant, got %s", ar.Role)
	}
	if ar.Model != "claude-sonnet-4-5" {
		t.Fatalf("expected model claude-sonnet-4-5, got %s", ar.Model)
	}
	if len(ar.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(ar.Content))
	}
	if ar.Content[0].Type != "text" {
		t.Fatalf("expected text block, got %s", ar.Content[0].Type)
	}
	if ar.Content[0].Text != "Hello!" {
		t.Fatalf("expected Hello!, got %s", ar.Content[0].Text)
	}
	if *ar.StopReason != "end_turn" {
		t.Fatalf("expected stop_reason end_turn, got %s", *ar.StopReason)
	}
	if ar.Usage.InputTokens != 10 {
		t.Fatalf("expected 10 input tokens, got %d", ar.Usage.InputTokens)
	}
	if ar.Usage.OutputTokens != 5 {
		t.Fatalf("expected 5 output tokens, got %d", ar.Usage.OutputTokens)
	}
}

func TestChatResponseToAnthropic_ToolCalls(t *testing.T) {
	toolCalls := "tool_calls"
	resp := &gateway.ChatResponse{
		ID: "chatcmpl-456",
		Choices: []gateway.Choice{{
			Index: 0,
			Message: &gateway.Message{
				Role:    "assistant",
				Content: "Let me check.",
				ToolCalls: []gateway.ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: gateway.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"city":"NYC"}`,
					},
				}},
			},
			FinishReason: &toolCalls,
		}},
		Usage: &gateway.Usage{PromptTokens: 10, CompletionTokens: 15, TotalTokens: 25},
	}

	ar := chatResponseToAnthropic(resp, "claude-sonnet-4-5")

	if len(ar.Content) != 2 {
		t.Fatalf("expected 2 content blocks (text + tool_use), got %d", len(ar.Content))
	}
	if ar.Content[0].Type != "text" {
		t.Fatalf("expected first block text, got %s", ar.Content[0].Type)
	}
	if ar.Content[1].Type != "tool_use" {
		t.Fatalf("expected second block tool_use, got %s", ar.Content[1].Type)
	}
	if ar.Content[1].ID != "call_1" {
		t.Fatalf("expected tool id call_1, got %s", ar.Content[1].ID)
	}
	if ar.Content[1].Name != "get_weather" {
		t.Fatalf("expected tool name get_weather, got %s", ar.Content[1].Name)
	}
	if *ar.StopReason != "tool_use" {
		t.Fatalf("expected stop_reason tool_use, got %s", *ar.StopReason)
	}
}

func TestChatResponseToAnthropic_EmptyContent(t *testing.T) {
	stop := "stop"
	resp := &gateway.ChatResponse{
		Choices: []gateway.Choice{{
			Index:        0,
			Message:      &gateway.Message{Role: "assistant"},
			FinishReason: &stop,
		}},
	}

	ar := chatResponseToAnthropic(resp, "claude-sonnet-4-5")

	if ar.Content == nil {
		t.Fatal("content should be empty slice, not nil")
	}
	if len(ar.Content) != 0 {
		t.Fatalf("expected 0 content blocks, got %d", len(ar.Content))
	}
}

func TestChatResponseToAnthropic_FinishReasonMapping(t *testing.T) {
	tests := []struct {
		openai    string
		anthropic string
	}{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"content_filter", "end_turn"},
	}

	for _, tt := range tests {
		t.Run(tt.openai, func(t *testing.T) {
			fr := tt.openai
			resp := &gateway.ChatResponse{
				Choices: []gateway.Choice{{
					Message:      &gateway.Message{Role: "assistant"},
					FinishReason: &fr,
				}},
			}
			ar := chatResponseToAnthropic(resp, "claude-sonnet-4-5")
			if *ar.StopReason != tt.anthropic {
				t.Fatalf("expected %s, got %s", tt.anthropic, *ar.StopReason)
			}
		})
	}
}

// ============================================================================
// responsesToChatRequest conversion tests
// ============================================================================

func TestResponsesToChatRequest_SimpleString(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: "Hello!",
	}

	cr := responsesToChatRequest(req)

	if cr.Model != "gpt-4o" {
		t.Fatalf("expected model gpt-4o, got %s", cr.Model)
	}
	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "user" {
		t.Fatalf("expected role user, got %s", cr.Messages[0].Role)
	}
	if cr.Messages[0].Content != "Hello!" {
		t.Fatalf("expected content Hello!, got %v", cr.Messages[0].Content)
	}
}

func TestResponsesToChatRequest_WithInstructions(t *testing.T) {
	req := &responsesRequest{
		Model:        "gpt-4o",
		Instructions: "You are a pirate.",
		Input:        "Hello!",
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "system" {
		t.Fatalf("expected system message first, got %s", cr.Messages[0].Role)
	}
	if cr.Messages[0].Content != "You are a pirate." {
		t.Fatalf("expected system content, got %v", cr.Messages[0].Content)
	}
}

func TestResponsesToChatRequest_MaxOutputTokens(t *testing.T) {
	max := 2048
	req := &responsesRequest{
		Model:           "gpt-4o",
		Input:           "hi",
		MaxOutputTokens: &max,
	}

	cr := responsesToChatRequest(req)

	if *cr.MaxTokens != 2048 {
		t.Fatalf("expected max_tokens 2048, got %d", *cr.MaxTokens)
	}
}

func TestResponsesToChatRequest_ReasoningEffort(t *testing.T) {
	req := &responsesRequest{
		Model:     "o3",
		Input:     "solve this",
		Reasoning: &respReasoning{Effort: "high"},
	}

	cr := responsesToChatRequest(req)

	if cr.ReasoningEffort != "high" {
		t.Fatalf("expected reasoning_effort high, got %s", cr.ReasoningEffort)
	}
}

func TestResponsesToChatRequest_InputItems(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": "first message",
			},
			map[string]any{
				"type":    "message",
				"role":    "assistant",
				"content": "ok",
			},
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": "second message",
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "user" {
		t.Fatalf("expected user, got %s", cr.Messages[0].Role)
	}
	if cr.Messages[1].Role != "assistant" {
		t.Fatalf("expected assistant, got %s", cr.Messages[1].Role)
	}
}

func TestResponsesToChatRequest_FunctionCallItems(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": "weather?",
			},
			map[string]any{
				"type":      "function_call",
				"call_id":   "call_123",
				"name":      "get_weather",
				"arguments": `{"city":"NYC"}`,
			},
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_123",
				"output":  "72°F",
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(cr.Messages))
	}

	// function_call → assistant with tool_calls
	if cr.Messages[1].Role != "assistant" {
		t.Fatalf("expected assistant for function_call, got %s", cr.Messages[1].Role)
	}
	if len(cr.Messages[1].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(cr.Messages[1].ToolCalls))
	}
	if cr.Messages[1].ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", cr.Messages[1].ToolCalls[0].Function.Name)
	}

	// function_call_output → tool
	if cr.Messages[2].Role != "tool" {
		t.Fatalf("expected tool for function_call_output, got %s", cr.Messages[2].Role)
	}
	if cr.Messages[2].ToolCallID != "call_123" {
		t.Fatalf("expected call_id call_123, got %s", cr.Messages[2].ToolCallID)
	}
	if cr.Messages[2].Content != "72°F" {
		t.Fatalf("expected output content, got %v", cr.Messages[2].Content)
	}
}

func TestResponsesToChatRequest_InputTextParts(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "Hello"},
					map[string]any{"type": "input_text", "text": "World"},
				},
			},
		},
	}

	cr := responsesToChatRequest(req)

	content := cr.Messages[0].Content.(string)
	if !strings.Contains(content, "Hello") || !strings.Contains(content, "World") {
		t.Fatalf("expected both text parts, got %s", content)
	}
}

func TestResponsesToChatRequest_Tools(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: "hi",
		Tools: []any{
			map[string]any{
				"type":        "function",
				"name":        "get_weather",
				"description": "Get weather",
				"parameters":  map[string]any{"type": "object"},
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cr.Tools))
	}
	if cr.Tools[0].Function.Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", cr.Tools[0].Function.Name)
	}
}

func TestResponsesToChatRequest_ResponseFormat(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: "hi",
		Text: &respText{
			Format: map[string]any{"type": "json_object"},
		},
	}

	cr := responsesToChatRequest(req)

	if cr.ResponseFormat == nil {
		t.Fatal("expected response_format to be set")
	}
	if cr.ResponseFormat.Type != "json_object" {
		t.Fatalf("expected json_object, got %s", cr.ResponseFormat.Type)
	}
}

// ============================================================================
// chatResponseToResponses conversion tests
// ============================================================================

func TestChatResponseToResponses_Basic(t *testing.T) {
	stop := "stop"
	resp := &gateway.ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []gateway.Choice{{
			Index:        0,
			Message:      &gateway.Message{Role: "assistant", Content: "Hello!"},
			FinishReason: &stop,
		}},
		Usage: &gateway.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	rr := chatResponseToResponses(resp, "gpt-4o")

	if rr.Object != "response" {
		t.Fatalf("expected object response, got %s", rr.Object)
	}
	if rr.Status != "completed" {
		t.Fatalf("expected status completed, got %s", rr.Status)
	}
	if rr.Model != "gpt-4o" {
		t.Fatalf("expected model gpt-4o, got %s", rr.Model)
	}
	if len(rr.Output) != 1 {
		t.Fatalf("expected 1 output, got %d", len(rr.Output))
	}
	if rr.Output[0].Type != "message" {
		t.Fatalf("expected message type, got %s", rr.Output[0].Type)
	}
	if len(rr.Output[0].Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(rr.Output[0].Content))
	}
	if rr.Output[0].Content[0].Type != "output_text" {
		t.Fatalf("expected output_text, got %s", rr.Output[0].Content[0].Type)
	}
	if rr.Output[0].Content[0].Text != "Hello!" {
		t.Fatalf("expected Hello!, got %s", rr.Output[0].Content[0].Text)
	}
	if rr.Usage.InputTokens != 10 {
		t.Fatalf("expected 10 input tokens, got %d", rr.Usage.InputTokens)
	}
	if rr.Usage.OutputTokens != 5 {
		t.Fatalf("expected 5 output tokens, got %d", rr.Usage.OutputTokens)
	}
}

func TestChatResponseToResponses_ToolCalls(t *testing.T) {
	toolCalls := "tool_calls"
	resp := &gateway.ChatResponse{
		ID: "chatcmpl-456",
		Choices: []gateway.Choice{{
			Index: 0,
			Message: &gateway.Message{
				Role:    "assistant",
				Content: "Let me check.",
				ToolCalls: []gateway.ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: gateway.ToolCallFunction{
						Name:      "get_weather",
						Arguments: `{"city":"NYC"}`,
					},
				}},
			},
			FinishReason: &toolCalls,
		}},
		Usage: &gateway.Usage{PromptTokens: 10, CompletionTokens: 15, TotalTokens: 25},
	}

	rr := chatResponseToResponses(resp, "gpt-4o")

	if len(rr.Output) != 2 {
		t.Fatalf("expected 2 outputs (message + function_call), got %d", len(rr.Output))
	}
	if rr.Output[0].Type != "message" {
		t.Fatalf("expected message, got %s", rr.Output[0].Type)
	}
	if rr.Output[1].Type != "function_call" {
		t.Fatalf("expected function_call, got %s", rr.Output[1].Type)
	}
	if rr.Output[1].Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", rr.Output[1].Name)
	}
	if rr.Output[1].Args != `{"city":"NYC"}` {
		t.Fatalf("expected args, got %s", rr.Output[1].Args)
	}
}

func TestChatResponseToResponses_LengthStatus(t *testing.T) {
	length := "length"
	resp := &gateway.ChatResponse{
		Choices: []gateway.Choice{{
			Message:      &gateway.Message{Role: "assistant", Content: "partial..."},
			FinishReason: &length,
		}},
	}

	rr := chatResponseToResponses(resp, "gpt-4o")

	if rr.Status != "incomplete" {
		t.Fatalf("expected status incomplete for length finish, got %s", rr.Status)
	}
}

// ============================================================================
// calculateCost tests
// ============================================================================

func TestCalculateCost_Basic(t *testing.T) {
	inputPrice := 2.50  // per million tokens
	outputPrice := 10.0 // per million tokens
	route := &gateway.RouteResult{
		InputPrice:  &inputPrice,
		OutputPrice: &outputPrice,
	}
	resp := &gateway.ChatResponse{
		Usage: &gateway.Usage{
			PromptTokens:     1000,
			CompletionTokens: 500,
		},
	}

	cost := calculateCost(route, resp)

	// 1000 * 2.50 / 1M + 500 * 10.0 / 1M = 0.0025 + 0.005 = 0.0075
	expected := 0.0075
	if cost < expected-0.0001 || cost > expected+0.0001 {
		t.Fatalf("expected cost ~%.4f, got %.4f", expected, cost)
	}
}

func TestCalculateCost_NilUsage(t *testing.T) {
	route := &gateway.RouteResult{}
	cost := calculateCost(route, nil)
	if cost != 0 {
		t.Fatalf("expected 0 for nil response, got %f", cost)
	}
}

func TestCalculateCost_NilPrices(t *testing.T) {
	route := &gateway.RouteResult{}
	resp := &gateway.ChatResponse{
		Usage: &gateway.Usage{PromptTokens: 100, CompletionTokens: 50},
	}

	cost := calculateCost(route, resp)
	if cost != 0 {
		t.Fatalf("expected 0 for nil prices, got %f", cost)
	}
}

func TestCalculateCost_NilResponse(t *testing.T) {
	inputPrice := 2.50
	route := &gateway.RouteResult{InputPrice: &inputPrice}

	cost := calculateCost(route, &gateway.ChatResponse{})
	if cost != 0 {
		t.Fatalf("expected 0 for nil usage, got %f", cost)
	}
}

// ============================================================================
// SSE format validation helpers
// ============================================================================

func TestAnthropicError_Format(t *testing.T) {
	// Verify the error format struct
	errType := "invalid_request_error"
	errMsg := "model is required"

	errBody := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errType,
			"message": errMsg,
		},
	}

	data, _ := json.Marshal(errBody)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	if parsed["type"] != "error" {
		t.Fatalf("expected type error, got %v", parsed["type"])
	}
	errObj := parsed["error"].(map[string]any)
	if errObj["type"] != errType {
		t.Fatalf("expected error type %s, got %v", errType, errObj["type"])
	}
}
