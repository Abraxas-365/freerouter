package gateway

import (
	"encoding/json"
	"fmt"
	"time"
)

// AnthropicTranslator converts between OpenAI format and Anthropic Messages API.
type AnthropicTranslator struct{}

// ---------- Request types (Anthropic native) ----------

type anthropicRequest struct {
	Model        string                `json:"model"`
	MaxTokens    int                   `json:"max_tokens"`
	System       any                   `json:"system,omitempty"`
	Messages     []anthropicMessage    `json:"messages"`
	Temperature  *float64              `json:"temperature,omitempty"`
	TopP         *float64              `json:"top_p,omitempty"`
	TopK         *int                  `json:"top_k,omitempty"`
	Stream       bool                  `json:"stream,omitempty"`
	StopSeqs     any                   `json:"stop_sequences,omitempty"`
	Tools        []anthropicTool       `json:"tools,omitempty"`
	ToolChoice   any                   `json:"tool_choice,omitempty"`
	Thinking     *anthropicThinking    `json:"thinking,omitempty"`
	OutputConfig *anthropicOutputConfig `json:"output_config,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

type anthropicOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

// defaultCacheControl is the cache_control value auto-injected on cache
// breakpoints. Anthropic's default TTL (5 minutes) applies.
var defaultCacheControl = map[string]any{"type": "ephemeral"}

// ---------- Response types (Anthropic native) ----------

type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason *string            `json:"stop_reason"`
	Usage      *anthropicUsage    `json:"usage"`
}

type anthropicContent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
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
	Message      *anthropicResponse `json:"message,omitempty"`
	Index        int                `json:"index,omitempty"`
	ContentBlock *anthropicContent  `json:"content_block,omitempty"`
	Delta        *anthropicDelta    `json:"delta,omitempty"`
	Usage        *anthropicUsage    `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string  `json:"type,omitempty"`
	Text        string  `json:"text,omitempty"`
	Thinking    string  `json:"thinking,omitempty"`
	Signature   string  `json:"signature,omitempty"`
	StopReason  *string `json:"stop_reason,omitempty"`
	PartialJSON string  `json:"partial_json,omitempty"`
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
		TopK:        req.TopK,
		Stream:      req.Stream,
		StopSeqs:    req.Stop,
	}

	// max_completion_tokens takes precedence over max_tokens (OpenAI reasoning model convention).
	if req.MaxCompletionTokens != nil {
		ar.MaxTokens = *req.MaxCompletionTokens
	} else if req.MaxTokens != nil {
		ar.MaxTokens = *req.MaxTokens
	} else {
		ar.MaxTokens = 4096
	}

	// Exact thinking passthrough (from /v1/messages) takes precedence over
	// reasoning_effort mapping (from /v1/chat/completions).
	if req.Thinking != nil {
		ar.Thinking = &anthropicThinking{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
			Display:      req.Thinking.Display,
		}
		if ar.Thinking.Type == "enabled" && ar.Thinking.BudgetTokens > 0 {
			if ar.MaxTokens <= ar.Thinking.BudgetTokens {
				ar.MaxTokens = ar.Thinking.BudgetTokens + 4096
			}
			ar.Temperature = nil
		}
		if ar.Thinking.Type == "adaptive" {
			ar.Temperature = nil
		}
		// Exact output_config passthrough from /v1/messages.
		if req.OutputConfig != nil {
			ar.OutputConfig = &anthropicOutputConfig{Effort: req.OutputConfig.Effort}
		}
	} else if req.ReasoningEffort != "" {
		// Map reasoning_effort → Anthropic thinking + output_config.effort.
		// Newer models (Opus 4.7+) use adaptive thinking + effort; older models
		// use enabled + budget_tokens. We set both so the request works either way.
		switch req.ReasoningEffort {
		case "none", "minimal":
			// No thinking
		case "low":
			ar.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: 1024}
			ar.OutputConfig = &anthropicOutputConfig{Effort: "low"}
		case "medium":
			ar.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: 8192}
			ar.OutputConfig = &anthropicOutputConfig{Effort: "medium"}
		case "high":
			ar.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: 16384}
			ar.OutputConfig = &anthropicOutputConfig{Effort: "high"}
		case "xhigh":
			ar.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: 32768}
			ar.OutputConfig = &anthropicOutputConfig{Effort: "xhigh"}
		case "max":
			ar.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: 65536}
			ar.OutputConfig = &anthropicOutputConfig{Effort: "max"}
		}

		if ar.Thinking != nil {
			// Anthropic requires max_tokens > budget_tokens.
			if ar.MaxTokens <= ar.Thinking.BudgetTokens {
				ar.MaxTokens = ar.Thinking.BudgetTokens + 4096
			}
			// Anthropic requires temperature to be unset (defaults to 1) when thinking is enabled.
			ar.Temperature = nil
		}
	}

	// Track whether any incoming message already has cache_control set.
	// If so, we skip auto-injection and respect the client's caching strategy.
	clientCaching := false

	// System prompt — convert to block array to support cache_control.
	for _, msg := range req.Messages {
		if msg.Role == "system" || msg.Role == "developer" {
			sysBlock := map[string]any{"type": "text"}
			if s, ok := msg.Content.(string); ok {
				sysBlock["text"] = s
			} else {
				sysBlock["text"] = msg.Content
			}
			if msg.CacheControl != nil {
				sysBlock["cache_control"] = msg.CacheControl
				clientCaching = true
			}
			ar.System = []map[string]any{sysBlock}
		}
	}

	var messages []anthropicMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" || msg.Role == "developer" {
			continue
		}

		am := anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Convert tool results
		if msg.Role == "tool" {
			am.Role = "user"
			contentStr, _ := msg.Content.(string)
			block := map[string]any{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     contentStr,
			}
			if msg.CacheControl != nil {
				block["cache_control"] = msg.CacheControl
				clientCaching = true
			}
			am.Content = []map[string]any{block}
		}

		// Convert assistant messages with tool_calls
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			var blocks []map[string]any
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
			if msg.CacheControl != nil {
				clientCaching = true
				if len(blocks) > 0 {
					blocks[len(blocks)-1]["cache_control"] = msg.CacheControl
				}
			}
			am.Content = blocks
		}

		// For user/assistant messages with cache_control but simple string content,
		// convert to block array so we can attach cache_control.
		if msg.CacheControl != nil && msg.Role != "tool" && (msg.Role != "assistant" || len(msg.ToolCalls) == 0) {
			clientCaching = true
			if s, ok := msg.Content.(string); ok {
				am.Content = []map[string]any{{
					"type":          "text",
					"text":          s,
					"cache_control": msg.CacheControl,
				}}
			}
			// If content is already a block array ([]any), cache_control may
			// already be embedded in the blocks — pass through as-is.
		}

		messages = append(messages, am)
	}
	ar.Messages = messages

	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			ar.Tools = append(ar.Tools, anthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			})
		}

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

	// Marshal the structured request, then apply cache_control via JSON
	// manipulation (auto-inject when client didn't specify any).
	out, err := json.Marshal(ar)
	if err != nil {
		return nil, err
	}

	if !clientCaching {
		out, err = injectAnthropicCacheControl(out)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// injectAnthropicCacheControl adds cache_control breakpoints to an Anthropic
// request when the client did not provide any. This follows the rness strategy:
//
//  1. Last system prompt block — caches the entire system prefix.
//  2. Last tool definition — caches the full tool array.
//  3. Second-to-last user message — caches recent conversation context.
//  4. Midpoint message (at len/3) for long histories (≥10 messages).
//
// Cache misses have zero cost; hits reduce input token charges significantly.
func injectAnthropicCacheControl(body []byte) ([]byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return body, nil // non-fatal: return original body
	}

	// 1. Last system block
	if system, ok := raw["system"]; ok {
		switch s := system.(type) {
		case string:
			// Convert plain string to block array with cache_control.
			raw["system"] = []any{map[string]any{
				"type":          "text",
				"text":          s,
				"cache_control": defaultCacheControl,
			}}
		case []any:
			if len(s) > 0 {
				if block, ok := s[len(s)-1].(map[string]any); ok {
					block["cache_control"] = defaultCacheControl
				}
			}
		}
	}

	// 2. Last tool definition
	if tools, ok := raw["tools"].([]any); ok && len(tools) > 0 {
		if lastTool, ok := tools[len(tools)-1].(map[string]any); ok {
			lastTool["cache_control"] = defaultCacheControl
		}
	}

	// 3. Message breakpoints
	if messages, ok := raw["messages"].([]any); ok {
		n := len(messages)
		// Second-to-last message (most recent context, highest cache value).
		if n >= 2 {
			markMessageCacheControl(messages[n-2])
		}
		// Midpoint breakpoint for long histories (≥10 messages, at len/3).
		if n >= 10 {
			mid := n / 3
			if mid > 0 {
				markMessageCacheControl(messages[mid])
			}
		}
	}

	return json.Marshal(raw)
}

// markMessageCacheControl adds cache_control to an Anthropic message.
// For string content, it converts to a block array. For block arrays,
// it marks the last block. For already-structured content, it adds to
// the last element.
func markMessageCacheControl(msg any) {
	m, ok := msg.(map[string]any)
	if !ok {
		return
	}
	content := m["content"]
	switch c := content.(type) {
	case string:
		// Convert to block array so we can attach cache_control.
		m["content"] = []any{map[string]any{
			"type":          "text",
			"text":          c,
			"cache_control": defaultCacheControl,
		}}
	case []any:
		if len(c) > 0 {
			if block, ok := c[len(c)-1].(map[string]any); ok {
				block["cache_control"] = defaultCacheControl
			}
		}
	}
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

	choice := Choice{Index: 0}
	msg := &Message{Role: "assistant"}

	var textParts []string
	var thinkingParts []string
	var toolCalls []ToolCall

	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "thinking":
			thinkingParts = append(thinkingParts, block.Thinking)
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
	if len(thinkingParts) > 0 {
		combined := ""
		for i, p := range thinkingParts {
			if i > 0 {
				combined += "\n"
			}
			combined += p
		}
		msg.Reasoning = combined
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
	var evt anthropicStreamEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, false, fmt.Errorf("failed to parse Anthropic stream event: %w", err)
	}

	switch evt.Type {
	case "message_start":
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
			// Anthropic sends input token counts (including cache metrics)
			// on the message_start event.
			if evt.Message.Usage != nil {
				chunk.Usage = &Usage{
					PromptTokens:            evt.Message.Usage.InputTokens,
					CacheReadInputTokens:    evt.Message.Usage.CacheReadInputTokens,
					CacheCreationInputToken: evt.Message.Usage.CacheCreationInputTokens,
				}
			}
		}
		out, err := json.Marshal(chunk)
		return out, false, err

	case "content_block_start":
		if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
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
								Arguments: "",
							},
						}},
					},
				}},
			}
			out, err := json.Marshal(chunk)
			return out, false, err
		}
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

		case "thinking_delta":
			chunk := ChatStreamChunk{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Choices: []Choice{{
					Index: 0,
					Delta: &Message{Reasoning: evt.Delta.Thinking},
				}},
			}
			out, err := json.Marshal(chunk)
			return out, false, err

		case "signature_delta":
			chunk := ChatStreamChunk{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Choices: []Choice{{
					Index: 0,
					Delta: &Message{Signature: evt.Delta.Signature},
				}},
			}
			out, err := json.Marshal(chunk)
			return out, false, err
		}
		return nil, false, nil

	case "message_delta":
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
		return nil, false, nil
	}
}

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
