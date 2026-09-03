package gateway

import (
	"encoding/json"
	"fmt"
	"time"
)

// AnthropicTranslator converts between OpenAI format and Anthropic Messages API.
//
// Key differences:
//   - System message is top-level, not in messages array
//   - max_tokens is required
//   - Response uses content blocks instead of a single content string
//   - Streaming uses event types: message_start, content_block_delta, message_delta, message_stop
//   - stop_reason instead of finish_reason
//   - Tool calls use tool_use content blocks
type AnthropicTranslator struct{}

// ---------- Request types (Anthropic native) ----------

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      any                `json:"system,omitempty"` // string or []contentBlock
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	StopSeqs    any                `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	ToolChoice  any                `json:"tool_choice,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentBlock
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

// ---------- Response types (Anthropic native) ----------

type anthropicResponse struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"` // "message"
	Role       string               `json:"role"`
	Content    []anthropicContent   `json:"content"`
	Model      string               `json:"model"`
	StopReason *string              `json:"stop_reason"`
	Usage      *anthropicUsage      `json:"usage"`
}

type anthropicContent struct {
	Type  string `json:"type"` // "text" or "tool_use"
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`    // tool_use
	Name  string `json:"name,omitempty"`  // tool_use
	Input any    `json:"input,omitempty"` // tool_use
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ---------- Streaming types ----------

type anthropicStreamEvent struct {
	Type         string             `json:"type"`
	Message      *anthropicResponse `json:"message,omitempty"`       // message_start
	Index        int                `json:"index,omitempty"`         // content_block_start/delta
	ContentBlock *anthropicContent  `json:"content_block,omitempty"` // content_block_start
	Delta        *anthropicDelta    `json:"delta,omitempty"`         // content_block_delta, message_delta
	Usage        *anthropicUsage    `json:"usage,omitempty"`         // message_delta
}

type anthropicDelta struct {
	Type       string  `json:"type,omitempty"` // "text_delta", "input_json_delta"
	Text       string  `json:"text,omitempty"`
	StopReason *string `json:"stop_reason,omitempty"` // in message_delta
	PartialJSON string `json:"partial_json,omitempty"`
}

// ============================================================================
// TransformRequest
// ============================================================================

func (t *AnthropicTranslator) TransformRequest(body []byte, model string) ([]byte, error) {
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	ar := anthropicRequest{
		Model:       model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		StopSeqs:    req.Stop,
	}

	// max_tokens is required for Anthropic
	if req.MaxTokens != nil {
		ar.MaxTokens = *req.MaxTokens
	} else {
		ar.MaxTokens = 4096
	}

	// Extract system messages and convert the rest
	var messages []anthropicMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			ar.System = msg.Content
			continue
		}

		am := anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool results: OpenAI "tool" role → Anthropic "user" role with tool_result content block
		if msg.Role == "tool" {
			am.Role = "user"
			contentStr, _ := msg.Content.(string)
			am.Content = []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     contentStr,
			}}
		}

		// Convert assistant messages with tool_calls → Anthropic tool_use content blocks
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var blocks []map[string]any

			// Add text content if present
			if contentStr, ok := msg.Content.(string); ok && contentStr != "" {
				blocks = append(blocks, map[string]any{
					"type": "text",
					"text": contentStr,
				})
			}

			for _, tc := range msg.ToolCalls {
				var input any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": input,
				})
			}
			am.Content = blocks
		}

		messages = append(messages, am)
	}
	ar.Messages = messages

	// Convert tools
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			ar.Tools = append(ar.Tools, anthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			})
		}

		// Convert tool_choice
		if req.ToolChoice != nil {
			switch v := req.ToolChoice.(type) {
			case string:
				switch v {
				case "auto":
					ar.ToolChoice = map[string]string{"type": "auto"}
				case "required":
					ar.ToolChoice = map[string]string{"type": "any"}
				case "none":
					ar.Tools = nil
					ar.ToolChoice = nil
				}
			case map[string]any:
				if fn, ok := v["function"].(map[string]any); ok {
					if name, ok := fn["name"].(string); ok {
						ar.ToolChoice = map[string]string{"type": "tool", "name": name}
					}
				}
			}
		}
	}

	return json.Marshal(ar)
}

// ============================================================================
// TransformResponse
// ============================================================================

func (t *AnthropicTranslator) TransformResponse(body []byte) (*ChatResponse, error) {
	var ar anthropicResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	resp := &ChatResponse{
		ID:      ar.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   ar.Model,
	}

	// Build choice from content blocks
	choice := Choice{Index: 0}
	msg := &Message{Role: "assistant"}

	var textParts []string
	var toolCalls []ToolCall

	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(args),
				},
			})
		}
	}

	if len(textParts) > 0 {
		combined := ""
		for i, p := range textParts {
			if i > 0 {
				combined += "\n"
			}
			combined += p
		}
		msg.Content = combined
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	choice.Message = msg
	choice.FinishReason = mapAnthropicStopReason(ar.StopReason)

	resp.Choices = []Choice{choice}

	if ar.Usage != nil {
		resp.Usage = &Usage{
			PromptTokens:            ar.Usage.InputTokens,
			CompletionTokens:        ar.Usage.OutputTokens,
			TotalTokens:             ar.Usage.InputTokens + ar.Usage.OutputTokens,
			CacheReadInputTokens:    ar.Usage.CacheReadInputTokens,
			CacheCreationInputToken: ar.Usage.CacheCreationInputTokens,
		}
	}

	return resp, nil
}

// ============================================================================
// TransformStreamEvent
// ============================================================================

func (t *AnthropicTranslator) TransformStreamEvent(data []byte) ([]byte, bool, error) {
	// Anthropic sends "event: <type>" lines followed by "data: <json>".
	// By the time this is called, the SSE framing is already stripped and we
	// receive the raw JSON payload.

	var evt anthropicStreamEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, false, fmt.Errorf("failed to parse Anthropic stream event: %w", err)
	}

	switch evt.Type {
	case "message_start":
		// First event — send the role delta
		chunk := ChatStreamChunk{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []Choice{{
				Index: 0,
				Delta: &Message{Role: "assistant"},
			}},
		}
		if evt.Message != nil {
			chunk.ID = evt.Message.ID
			chunk.Model = evt.Message.Model
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	case "content_block_start":
		if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
			args := ""
			chunk := ChatStreamChunk{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Choices: []Choice{{
					Index: 0,
					Delta: &Message{
						ToolCalls: []ToolCall{{
							ID:   evt.ContentBlock.ID,
							Type: "function",
							Function: ToolCallFunction{
								Name:      evt.ContentBlock.Name,
								Arguments: args,
							},
						}},
					},
				}},
			}
			out, err := json.Marshal(chunk)
			return out, false, err
		}
		// text block start — skip, content comes in deltas
		return nil, false, nil

	case "content_block_delta":
		if evt.Delta == nil {
			return nil, false, nil
		}

		switch evt.Delta.Type {
		case "text_delta":
			chunk := ChatStreamChunk{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Choices: []Choice{{
					Index: 0,
					Delta: &Message{Content: evt.Delta.Text},
				}},
			}
			out, err := json.Marshal(chunk)
			return out, false, err

		case "input_json_delta":
			chunk := ChatStreamChunk{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Choices: []Choice{{
					Index: 0,
					Delta: &Message{
						ToolCalls: []ToolCall{{
							Function: ToolCallFunction{
								Arguments: evt.Delta.PartialJSON,
							},
						}},
					},
				}},
			}
			out, err := json.Marshal(chunk)
			return out, false, err
		}
		return nil, false, nil

	case "message_delta":
		// Final delta with stop_reason and output token count
		chunk := ChatStreamChunk{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []Choice{{
				Index:        0,
				Delta:        &Message{},
				FinishReason: mapAnthropicStopReason(evt.Delta.StopReason),
			}},
		}
		if evt.Usage != nil {
			chunk.Usage = &Usage{
				CompletionTokens: evt.Usage.OutputTokens,
			}
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	case "message_stop":
		return nil, true, nil

	case "content_block_stop", "ping":
		return nil, false, nil

	default:
		// Unknown event type — skip
		return nil, false, nil
	}
}

// mapAnthropicStopReason converts Anthropic stop_reason to OpenAI finish_reason
func mapAnthropicStopReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	var mapped string
	switch *reason {
	case "end_turn":
		mapped = "stop"
	case "max_tokens":
		mapped = "length"
	case "tool_use":
		mapped = "tool_calls"
	case "stop_sequence":
		mapped = "stop"
	default:
		mapped = *reason
	}
	return &mapped
}
