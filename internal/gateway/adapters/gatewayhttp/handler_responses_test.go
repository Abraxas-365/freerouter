package gatewayhttp

import (
	"encoding/json"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/gateway"
)

func TestResponsesToChatRequest_StringInput(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: "Hello there",
	}

	cr := responsesToChatRequest(req)

	if cr.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %q", cr.Model)
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

func TestResponsesToChatRequest_Instructions(t *testing.T) {
	req := &responsesRequest{
		Model:        "gpt-4o",
		Instructions: "Be concise.",
		Input:        "Hi",
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "system" {
		t.Errorf("expected first message role system, got %q", cr.Messages[0].Role)
	}
	if text, ok := cr.Messages[0].Content.(string); !ok || text != "Be concise." {
		t.Errorf("unexpected system content: %v", cr.Messages[0].Content)
	}
}

func TestResponsesToChatRequest_ArrayInputMessage(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{"type": "message", "role": "user", "content": "hello"},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	if cr.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %q", cr.Messages[0].Role)
	}
	if text, ok := cr.Messages[0].Content.(string); !ok || text != "hello" {
		t.Errorf("expected content hello, got %v", cr.Messages[0].Content)
	}
}

func TestResponsesToChatRequest_ArrayInputContentParts(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{
				"type": "message", "role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "part one"},
					map[string]any{"type": "input_text", "text": "part two"},
				},
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	want := "part one\npart two"
	if text, ok := cr.Messages[0].Content.(string); !ok || text != want {
		t.Errorf("expected content %q, got %v", want, cr.Messages[0].Content)
	}
}

func TestResponsesToChatRequest_FunctionCall(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{
				"type": "function_call", "call_id": "call_1",
				"name": "get_weather", "arguments": `{"location":"NYC"}`,
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	m := cr.Messages[0]
	if m.Role != "assistant" {
		t.Errorf("expected role assistant, got %q", m.Role)
	}
	if len(m.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(m.ToolCalls))
	}
	if m.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected call ID call_1, got %q", m.ToolCalls[0].ID)
	}
	if m.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected function name get_weather, got %q", m.ToolCalls[0].Function.Name)
	}
}

func TestResponsesToChatRequest_FunctionCallOutput(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: []any{
			map[string]any{
				"type": "function_call_output", "call_id": "call_1",
				"output": "Sunny, 72F",
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(cr.Messages))
	}
	m := cr.Messages[0]
	if m.Role != "tool" {
		t.Errorf("expected role tool, got %q", m.Role)
	}
	if m.ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id call_1, got %q", m.ToolCallID)
	}
	if text, ok := m.Content.(string); !ok || text != "Sunny, 72F" {
		t.Errorf("expected content Sunny, 72F, got %v", m.Content)
	}
}

func TestResponsesToChatRequest_Reasoning(t *testing.T) {
	req := &responsesRequest{
		Model:     "o3",
		Input:     "solve this",
		Reasoning: &respReasoning{Effort: "high"},
	}

	cr := responsesToChatRequest(req)

	if cr.ReasoningEffort != "high" {
		t.Errorf("expected reasoning effort high, got %q", cr.ReasoningEffort)
	}
}

func TestResponsesToChatRequest_TextFormat(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: "hi",
		Text: &respText{
			Format: map[string]any{
				"type":   "json_schema",
				"schema": map[string]any{"type": "object"},
			},
		},
	}

	cr := responsesToChatRequest(req)

	if cr.ResponseFormat == nil {
		t.Fatal("expected response format to be set")
	}
	if cr.ResponseFormat.Type != "json_schema" {
		t.Errorf("expected type json_schema, got %q", cr.ResponseFormat.Type)
	}
	if cr.ResponseFormat.JSONSchema == nil {
		t.Error("expected JSONSchema to be set")
	}
}

func TestResponsesToChatRequest_Tools(t *testing.T) {
	req := &responsesRequest{
		Model: "gpt-4o",
		Input: "hi",
		Tools: []any{
			map[string]any{
				"type": "function", "name": "get_weather",
				"description": "Get the weather", "parameters": map[string]any{"type": "object"},
			},
		},
	}

	cr := responsesToChatRequest(req)

	if len(cr.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(cr.Tools))
	}
	if cr.Tools[0].Function.Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %q", cr.Tools[0].Function.Name)
	}
}

func TestResponsesToChatRequest_MaxOutputTokens(t *testing.T) {
	maxTokens := 256
	req := &responsesRequest{
		Model:           "gpt-4o",
		Input:           "hi",
		MaxOutputTokens: &maxTokens,
	}

	cr := responsesToChatRequest(req)

	if cr.MaxTokens == nil || *cr.MaxTokens != maxTokens {
		t.Errorf("expected max_tokens %d, got %v", maxTokens, cr.MaxTokens)
	}
}

func TestChatResponseToResponses_TextResponse(t *testing.T) {
	stopReason := "stop"
	resp := &gateway.ChatResponse{
		ID:    "chatcmpl-abc",
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

	rr := chatResponseToResponses(resp, "gpt-4o")

	if rr.Object != "response" {
		t.Errorf("expected object response, got %q", rr.Object)
	}
	if rr.Status != "completed" {
		t.Errorf("expected status completed, got %q", rr.Status)
	}
	if rr.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %q", rr.Model)
	}
	if len(rr.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(rr.Output))
	}
	out := rr.Output[0]
	if out.Type != "message" || out.Role != "assistant" || out.Status != "completed" {
		t.Errorf("unexpected output item: %+v", out)
	}
	if len(out.Content) != 1 || out.Content[0].Type != "output_text" || out.Content[0].Text != "Hello!" {
		t.Errorf("unexpected content: %+v", out.Content)
	}
	if rr.Usage == nil || rr.Usage.InputTokens != 10 || rr.Usage.OutputTokens != 5 || rr.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", rr.Usage)
	}
}

func TestChatResponseToResponses_IncompleteOnLength(t *testing.T) {
	finishReason := "length"
	resp := &gateway.ChatResponse{
		ID:    "chatcmpl-abc",
		Model: "gpt-real",
		Choices: []gateway.Choice{
			{Index: 0, Message: &gateway.Message{Role: "assistant", Content: "trunc"}, FinishReason: &finishReason},
		},
	}

	rr := chatResponseToResponses(resp, "gpt-4o")

	if rr.Status != "incomplete" {
		t.Errorf("expected status incomplete, got %q", rr.Status)
	}
}

func TestChatResponseToResponses_FunctionCallOutput(t *testing.T) {
	resp := &gateway.ChatResponse{
		ID:    "chatcmpl-abc",
		Model: "gpt-real",
		Choices: []gateway.Choice{
			{
				Index: 0,
				Message: &gateway.Message{
					Role: "assistant",
					ToolCalls: []gateway.ToolCall{
						{ID: "call_1", Type: "function", Function: gateway.ToolCallFunction{Name: "get_weather", Arguments: `{"location":"NYC"}`}},
					},
				},
			},
		},
	}

	rr := chatResponseToResponses(resp, "gpt-4o")

	if len(rr.Output) != 2 {
		t.Fatalf("expected 2 output items (message + function_call), got %d", len(rr.Output))
	}
	fc := rr.Output[1]
	if fc.Type != "function_call" {
		t.Errorf("expected type function_call, got %q", fc.Type)
	}
	if fc.CallID != "call_1" {
		t.Errorf("expected call_id call_1, got %q", fc.CallID)
	}
	if fc.Name != "get_weather" {
		t.Errorf("expected name get_weather, got %q", fc.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(fc.Args), &args); err != nil {
		t.Fatalf("failed to unmarshal args: %v", err)
	}
	if args["location"] != "NYC" {
		t.Errorf("expected location NYC, got %v", args["location"])
	}
}

func TestExtractResponsesTexts(t *testing.T) {
	req := &responsesRequest{
		Instructions: "Be helpful.",
		Input:        "What's the weather?",
	}

	texts := extractResponsesTexts(req)

	if len(texts) != 2 {
		t.Fatalf("expected 2 texts, got %d: %v", len(texts), texts)
	}
	if texts[0] != "Be helpful." || texts[1] != "What's the weather?" {
		t.Errorf("unexpected texts: %v", texts)
	}
}

func TestExtractResponsesTexts_ArrayInputIgnored(t *testing.T) {
	req := &responsesRequest{
		Instructions: "Be helpful.",
		Input:        []any{map[string]any{"type": "message", "role": "user", "content": "hi"}},
	}

	texts := extractResponsesTexts(req)

	if len(texts) != 1 || texts[0] != "Be helpful." {
		t.Errorf("expected only instructions text, got %v", texts)
	}
}
