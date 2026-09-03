package gateway

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CohereTranslator converts between OpenAI format and the Cohere v2 chat API.
//
// Key differences:
//   - Response content is a list of content blocks under message.content
//   - finish_reason uses upper-case values (COMPLETE, MAX_TOKENS, TOOL_CALL)
//   - Usage is nested under usage.billed_units
//   - Streaming uses typed events: message-start, content-delta, message-end
type CohereTranslator struct{}

// ---------- Request types (Cohere v2 native) ----------

type cohereRequest struct {
	Model       string          `json:"model"`
	Messages    []cohereMessage `json:"messages"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	P           *float64        `json:"p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	StopSeqs    any             `json:"stop_sequences,omitempty"`
	Tools       []Tool          `json:"tools,omitempty"` // Cohere v2 uses OpenAI-style tools
}

type cohereMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ---------- Response types (Cohere v2 native) ----------

type cohereResponse struct {
	ID           string              `json:"id"`
	FinishReason string              `json:"finish_reason"`
	Message      cohereRespMessage   `json:"message"`
	Usage        *cohereUsageWrapper `json:"usage,omitempty"`
}

type cohereRespMessage struct {
	Role      string          `json:"role"`
	Content   []cohereContent `json:"content"`
	ToolCalls []ToolCall      `json:"tool_calls,omitempty"`
}

type cohereContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text,omitempty"`
}

type cohereUsageWrapper struct {
	BilledUnits *cohereUsageUnits `json:"billed_units,omitempty"`
	Tokens      *cohereUsageUnits `json:"tokens,omitempty"`
}

type cohereUsageUnits struct {
	InputTokens  float64 `json:"input_tokens"`
	OutputTokens float64 `json:"output_tokens"`
}

// ---------- Streaming types ----------

type cohereStreamEvent struct {
	ID    string            `json:"id,omitempty"`
	Type  string            `json:"type"`
	Index int               `json:"index,omitempty"`
	Delta *cohereEventDelta `json:"delta,omitempty"`
}

type cohereEventDelta struct {
	Message      *cohereDeltaMessage `json:"message,omitempty"`
	FinishReason string              `json:"finish_reason,omitempty"` // message-end
	Usage        *cohereUsageWrapper `json:"usage,omitempty"`         // message-end
}

type cohereDeltaMessage struct {
	Role      string               `json:"role,omitempty"`
	Content   *cohereContent       `json:"content,omitempty"`
	ToolCalls *cohereDeltaToolCall `json:"tool_calls,omitempty"`
}

type cohereDeltaToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function *struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// ============================================================================
// TransformRequest
// ============================================================================

func (t *CohereTranslator) TransformRequest(body []byte, model string) ([]byte, error) {
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	cr := cohereRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		P:           req.TopP,
		Stream:      req.Stream,
		StopSeqs:    req.Stop,
		Tools:       req.Tools,
	}

	for _, msg := range req.Messages {
		cr.Messages = append(cr.Messages, cohereMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		})
	}

	return json.Marshal(cr)
}

// ============================================================================
// TransformResponse
// ============================================================================

func (t *CohereTranslator) TransformResponse(body []byte) (*ChatResponse, error) {
	var cr cohereResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return nil, fmt.Errorf("failed to parse Cohere response: %w", err)
	}

	msg := &Message{Role: "assistant"}

	var textParts []string
	for _, block := range cr.Message.Content {
		if block.Type == "text" {
			textParts = append(textParts, block.Text)
		}
	}
	if len(textParts) > 0 {
		msg.Content = strings.Join(textParts, "\n")
	}
	if len(cr.Message.ToolCalls) > 0 {
		msg.ToolCalls = cr.Message.ToolCalls
	}

	finish := mapCohereFinishReason(cr.FinishReason)
	resp := &ChatResponse{
		ID:      cr.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: []Choice{{
			Index:        0,
			Message:      msg,
			FinishReason: finish,
		}},
	}

	if cr.Usage != nil && cr.Usage.BilledUnits != nil {
		in := int(cr.Usage.BilledUnits.InputTokens)
		out := int(cr.Usage.BilledUnits.OutputTokens)
		resp.Usage = &Usage{
			PromptTokens:     in,
			CompletionTokens: out,
			TotalTokens:      in + out,
		}
	}

	return resp, nil
}

// ============================================================================
// TransformStreamEvent
// ============================================================================

func (t *CohereTranslator) TransformStreamEvent(data []byte) ([]byte, bool, error) {
	var evt cohereStreamEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, false, fmt.Errorf("failed to parse Cohere stream event: %w", err)
	}

	switch evt.Type {
	case "message-start":
		chunk := ChatStreamChunk{
			ID:      evt.ID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []Choice{{
				Index: 0,
				Delta: &Message{Role: "assistant"},
			}},
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	case "content-delta":
		if evt.Delta == nil || evt.Delta.Message == nil || evt.Delta.Message.Content == nil {
			return nil, false, nil
		}
		chunk := ChatStreamChunk{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []Choice{{
				Index: 0,
				Delta: &Message{Content: evt.Delta.Message.Content.Text},
			}},
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	case "tool-call-start", "tool-call-delta":
		if evt.Delta == nil || evt.Delta.Message == nil || evt.Delta.Message.ToolCalls == nil {
			return nil, false, nil
		}
		tc := evt.Delta.Message.ToolCalls
		call := ToolCall{ID: tc.ID, Type: "function"}
		if tc.Function != nil {
			call.Function = ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
		chunk := ChatStreamChunk{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []Choice{{
				Index: 0,
				Delta: &Message{ToolCalls: []ToolCall{call}},
			}},
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	case "message-end":
		chunk := ChatStreamChunk{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []Choice{{
				Index: 0,
				Delta: &Message{},
			}},
		}
		if evt.Delta != nil {
			chunk.Choices[0].FinishReason = mapCohereFinishReason(evt.Delta.FinishReason)
			if evt.Delta.Usage != nil && evt.Delta.Usage.BilledUnits != nil {
				in := int(evt.Delta.Usage.BilledUnits.InputTokens)
				out := int(evt.Delta.Usage.BilledUnits.OutputTokens)
				chunk.Usage = &Usage{
					PromptTokens:     in,
					CompletionTokens: out,
					TotalTokens:      in + out,
				}
			}
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	default:
		// content-start, content-end, tool-plan-delta, etc. — no output
		return nil, false, nil
	}
}

// mapCohereFinishReason converts Cohere finish_reason to OpenAI finish_reason
func mapCohereFinishReason(reason string) *string {
	if reason == "" {
		return nil
	}
	var mapped string
	switch reason {
	case "COMPLETE", "STOP_SEQUENCE":
		mapped = "stop"
	case "MAX_TOKENS":
		mapped = "length"
	case "TOOL_CALL":
		mapped = "tool_calls"
	default:
		mapped = strings.ToLower(reason)
	}
	return &mapped
}
