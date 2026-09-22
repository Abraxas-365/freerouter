package gatewayhttp

import (
	"encoding/json"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/gateway"
)

func TestAnthropicToChatRequest_TextMessage(t *testing.T) {
	maxTokens := 512
	req := &anthropicMessagesRequest{
		Model:     "claude-3-opus",
		MaxTokens: maxTokens,
		Messages: []anthropicMsg{
			{Role: "user", Content: "Hello there"},
		},
	}

	cr := anthropicToChatRequest(req)

	if cr.Model != "claude-3-opus" {
		t.Errorf("expected model claude-3-opus, got %q", cr.Model)
	}
	if cr.MaxTokens == nil || *cr.MaxTokens != maxTokens {
		t.Errorf("expected max_tokens %d, got %v", maxTokens, cr.MaxTokens)
	}
	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %q", cr.Messages[0].Role)
	}
	if text, ok := cr.Messages[0].Content.(string); !ok || text != "Hello there" {
		t.Errorf("expected content %q, got %v", "Hello there", cr.Messages[0].Content)
	}
}

func TestAnthropicToChatRequest_SystemMessage(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-3-opus",
		MaxTokens: 100,
		System:    "You are a helpful assistant.",
		Messages: []anthropicMsg{
			{Role: "user", Content: "Hi"},
		},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "system" {
		t.Errorf("expected first message role system, got %q", cr.Messages[0].Role)
	}
	if text, ok := cr.Messages[0].Content.(string); !ok || text != "You are a helpful assistant." {
		t.Errorf("unexpected system content: %v", cr.Messages[0].Content)
	}
}

func TestAnthropicToChatRequest_SystemBlocks(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-3-opus",
		MaxTokens: 100,
		System: []any{
			map[string]any{"type": "text", "text": "Part one."},
			map[string]any{"type": "text", "text": "Part two."},
		},
		Messages: []anthropicMsg{{Role: "user", Content: "Hi"}},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(cr.Messages))
	}
	want := "Part one.\nPart two."
	if text, ok := cr.Messages[0].Content.(string); !ok || text != want {
		t.Errorf("expected system content %q, got %v", want, cr.Messages[0].Content)
	}
}

func TestAnthropicToChatRequest_ToolUse(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-3-opus",
		MaxTokens: 100,
		Messages: []anthropicMsg{
			{
				Role: "assistant",
				Content: []any{
					map[string]any{"type": "text", "text": "Let me check the weather."},
					map[string]any{
						"type":  "tool_use",
						"id":    "toolu_123",
						"name":  "get_weather",
						"input": map[string]any{"location": "NYC"},
					},
				},
			},
		},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	m := cr.Messages[0]
	if len(m.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(m.ToolCalls))
	}
	if m.ToolCalls[0].ID != "toolu_123" {
		t.Errorf("expected tool call ID toolu_123, got %q", m.ToolCalls[0].ID)
	}
	if m.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected function name get_weather, got %q", m.ToolCalls[0].Function.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(m.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("failed to unmarshal tool call arguments: %v", err)
	}
	if args["location"] != "NYC" {
		t.Errorf("expected location NYC, got %v", args["location"])
	}
	if text, ok := m.Content.(string); !ok || text != "Let me check the weather." {
		t.Errorf("expected accompanying text content, got %v", m.Content)
	}
}

func TestAnthropicToChatRequest_ToolResult(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-3-opus",
		MaxTokens: 100,
		Messages: []anthropicMsg{
			{
				Role: "user",
				Content: []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "toolu_123",
						"content":     "Sunny, 72F",
					},
				},
			},
		},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	m := cr.Messages[0]
	if m.Role != "tool" {
		t.Errorf("expected role tool, got %q", m.Role)
	}
	if m.ToolCallID != "toolu_123" {
		t.Errorf("expected tool_call_id toolu_123, got %q", m.ToolCallID)
	}
	if text, ok := m.Content.(string); !ok || text != "Sunny, 72F" {
		t.Errorf("expected tool result content, got %v", m.Content)
	}
}

func TestAnthropicToChatRequest_Tools(t *testing.T) {
	req := &anthropicMessagesRequest{
		Model:     "claude-3-opus",
		MaxTokens: 100,
		Messages:  []anthropicMsg{{Role: "user", Content: "hi"}},
		Tools: []anthropicToolDef{
			{
				Name:        "get_weather",
				Description: "Get the weather",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}

	cr := anthropicToChatRequest(req)

	if len(cr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cr.Tools))
	}
	if cr.Tools[0].Function.Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %q", cr.Tools[0].Function.Name)
	}
}

func TestChatResponseToAnthropic_TextResponse(t *testing.T) {
	stopReason := "stop"
	resp := &gateway.ChatResponse{
		ID:    "chatcmpl-abc123",
		Model: "gpt-real",
		Choices: []gateway.Choice{
			{
				Index:        0,
				Message:      &gateway.Message{Role: "assistant", Content: "Hello!"},
				FinishReason: &stopReason,
			},
		},
		Usage: &gateway.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	ar := chatResponseToAnthropic(resp, "claude-3-opus")

	if ar.Type != "message" {
		t.Errorf("expected type message, got %q", ar.Type)
	}
	if ar.Role != "assistant" {
		t.Errorf("expected role assistant, got %q", ar.Role)
	}
	if ar.Model != "claude-3-opus" {
		t.Errorf("expected model claude-3-opus, got %q", ar.Model)
	}
	if len(ar.Content) != 1 || ar.Content[0].Type != "text" || ar.Content[0].Text != "Hello!" {
		t.Fatalf("unexpected content: %+v", ar.Content)
	}
	if ar.StopReason == nil || *ar.StopReason != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %v", ar.StopReason)
	}
	if ar.Usage == nil || ar.Usage.InputTokens != 10 || ar.Usage.OutputTokens != 5 {
		t.Fatalf("unexpected usage: %+v", ar.Usage)
	}
}

func TestChatResponseToAnthropic_ToolCallResponse(t *testing.T) {
	finishReason := "tool_calls"
	resp := &gateway.ChatResponse{
		ID:    "chatcmpl-xyz",
		Model: "gpt-real",
		Choices: []gateway.Choice{
			{
				Index: 0,
				Message: &gateway.Message{
					Role: "assistant",
					ToolCalls: []gateway.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: gateway.ToolCallFunction{
								Name:      "get_weather",
								Arguments: `{"location":"NYC"}`,
							},
						},
					},
				},
				FinishReason: &finishReason,
			},
		},
	}

	ar := chatResponseToAnthropic(resp, "claude-3-opus")

	if len(ar.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(ar.Content))
	}
	block := ar.Content[0]
	if block.Type != "tool_use" {
		t.Errorf("expected type tool_use, got %q", block.Type)
	}
	if block.Name != "get_weather" {
		t.Errorf("expected name get_weather, got %q", block.Name)
	}
	inputMap, ok := block.Input.(map[string]any)
	if !ok {
		t.Fatalf("expected input to be a map, got %T", block.Input)
	}
	if inputMap["location"] != "NYC" {
		t.Errorf("expected location NYC, got %v", inputMap["location"])
	}
	if ar.StopReason == nil || *ar.StopReason != "tool_use" {
		t.Errorf("expected stop_reason tool_use, got %v", ar.StopReason)
	}
}

func TestChatResponseToAnthropic_IDPrefixTranslation(t *testing.T) {
	resp := &gateway.ChatResponse{
		ID:    "chatcmpl-abc",
		Model: "gpt-real",
		Choices: []gateway.Choice{
			{Index: 0, Message: &gateway.Message{Role: "assistant", Content: "hi"}},
		},
	}

	ar := chatResponseToAnthropic(resp, "claude-3-opus")

	if ar.ID != "msg_abc" {
		t.Errorf("expected ID msg_abc, got %q", ar.ID)
	}
}

func TestExtractAnthropicTexts(t *testing.T) {
	messages := []anthropicMsg{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: []any{map[string]any{"type": "text", "text": "ignored"}}},
		{Role: "user", Content: "world"},
	}

	texts := extractAnthropicTexts(messages)

	if len(texts) != 2 {
		t.Fatalf("expected 2 string-content texts, got %d: %v", len(texts), texts)
	}
	if texts[0] != "hello" || texts[1] != "world" {
		t.Errorf("unexpected texts: %v", texts)
	}
}

func TestMapFinishReasonToAnthropic(t *testing.T) {
	cases := map[string]string{
		"stop":           "end_turn",
		"length":         "max_tokens",
		"tool_calls":     "tool_use",
		"content_filter": "end_turn",
		"unknown":        "end_turn",
	}
	for reason, want := range cases {
		r := reason
		got := mapFinishReasonToAnthropic(&r)
		if got != want {
			t.Errorf("mapFinishReasonToAnthropic(%q) = %q, want %q", reason, got, want)
		}
	}
	if got := mapFinishReasonToAnthropic(nil); got != "end_turn" {
		t.Errorf("mapFinishReasonToAnthropic(nil) = %q, want end_turn", got)
	}
}
