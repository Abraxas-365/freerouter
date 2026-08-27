package gatewayapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

// ============================================================================
// Anthropic Messages API types (client-facing)
// ============================================================================

type anthropicMessagesRequest struct {
	Model         string              `json:"model"`
	Messages      []anthropicMsg      `json:"messages"`
	System        any                 `json:"system,omitempty"` // string or []block
	MaxTokens     int                 `json:"max_tokens"`
	Temperature   *float64            `json:"temperature,omitempty"`
	TopP          *float64            `json:"top_p,omitempty"`
	Stream        bool                `json:"stream,omitempty"`
	StopSequences any                 `json:"stop_sequences,omitempty"`
	Tools         []anthropicToolDef  `json:"tools,omitempty"`
	ToolChoice    any                 `json:"tool_choice,omitempty"`
	Metadata      any                 `json:"metadata,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []contentBlock
}

type anthropicToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type anthropicMsgResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"` // "message"
	Role       string                  `json:"role"`
	Model      string                  `json:"model"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason *string                 `json:"stop_reason"`
	Usage      *anthropicMsgUsage      `json:"usage"`
}

type anthropicContentBlock struct {
	Type  string `json:"type"` // "text" or "tool_use"
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type anthropicMsgUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ============================================================================
// Handler
// ============================================================================

// AnthropicMessages handles POST /v1/messages (Anthropic Messages API).
func (h *GatewayHandlers) AnthropicMessages(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return anthropicError(c, http.StatusUnauthorized, "authentication_error", "unauthorized")
	}

	var req anthropicMessagesRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return anthropicError(c, http.StatusBadRequest, "invalid_request_error", "invalid request body")
	}
	if req.Model == "" {
		return anthropicError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
	}
	if req.MaxTokens == 0 {
		return anthropicError(c, http.StatusBadRequest, "invalid_request_error", "max_tokens is required")
	}

	requestedModel := req.Model

	// Convert to internal ChatRequest
	chatReq := anthropicToChatRequest(&req)

	// Resolve route
	tenantID := &authCtx.TenantID
	maxTokens := req.MaxTokens
	route, err := h.router.Resolve(c.Context(), chatReq.Model, tenantID, &maxTokens)
	if err != nil {
		return anthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
	}

	chatReq.Model = route.ExternalID
	body, _ := json.Marshal(chatReq)

	if req.Stream {
		return h.handleAnthropicStream(c, route, requestedModel, authCtx.TenantID, body)
	}
	return h.handleAnthropicNonStream(c, route, requestedModel, authCtx.TenantID, body)
}

func (h *GatewayHandlers) handleAnthropicNonStream(c *fiber.Ctx, route *gateway.RouteResult, requestedModel string, tenantID kernel.TenantID, body []byte) error {
	start := time.Now()

	resp, statusCode, err := h.upstream.Call(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.usage.LogRequest(tenantID, route, requestedModel, nil, statusCode, duration, false, err)
		return anthropicError(c, http.StatusBadGateway, "api_error", err.Error())
	}

	cost := calculateCost(route, resp)
	if cost > 0 {
		if _, err := h.billing.DebitUsage(c.Context(), tenantID, cost, ""); err != nil {
			slog.Error("failed to debit usage", "tenant_id", tenantID, "cost", cost, "error", err)
		}
	}

	h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, false, nil)

	// Convert OpenAI response → Anthropic response
	anthropicResp := chatResponseToAnthropic(resp, requestedModel)
	return c.JSON(anthropicResp)
}

func (h *GatewayHandlers) handleAnthropicStream(c *fiber.Ctx, route *gateway.RouteResult, requestedModel string, tenantID kernel.TenantID, body []byte) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		start := time.Now()
		var accumulatedUsage *gateway.Usage
		var lastFinishReason *string
		msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

		// message_start event
		startEvt := map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":      msgID,
				"type":    "message",
				"role":    "assistant",
				"model":   requestedModel,
				"content": []any{},
				"usage":   map[string]int{"input_tokens": 0, "output_tokens": 0},
			},
		}
		writeAnthropicSSE(w, "message_start", startEvt)

		// content_block_start for text
		writeAnthropicSSE(w, "content_block_start", map[string]any{
			"type":          "content_block_start",
			"index":         0,
			"content_block": map[string]string{"type": "text", "text": ""},
		})

		streamErr := h.upstream.Stream(c.Context(), route, body, func(chunk []byte) error {
			var streamChunk gateway.ChatStreamChunk
			if err := json.Unmarshal(chunk, &streamChunk); err != nil {
				return nil // skip unparseable chunks
			}

			if streamChunk.Usage != nil {
				accumulatedUsage = streamChunk.Usage
			}

			for _, choice := range streamChunk.Choices {
				if choice.FinishReason != nil {
					lastFinishReason = choice.FinishReason
				}
				if choice.Delta != nil && choice.Delta.Content != nil {
					text, ok := choice.Delta.Content.(string)
					if ok && text != "" {
						writeAnthropicSSE(w, "content_block_delta", map[string]any{
							"type":  "content_block_delta",
							"index": 0,
							"delta": map[string]string{"type": "text_delta", "text": text},
						})
					}
				}
			}
			return nil
		})

		duration := time.Since(start)

		// content_block_stop
		writeAnthropicSSE(w, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": 0,
		})

		// message_delta with stop_reason and usage
		stopReason := mapFinishReasonToAnthropic(lastFinishReason)
		deltaEvt := map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": stopReason},
		}
		if accumulatedUsage != nil {
			deltaEvt["usage"] = map[string]int{"output_tokens": accumulatedUsage.CompletionTokens}
		}
		writeAnthropicSSE(w, "message_delta", deltaEvt)

		// message_stop
		writeAnthropicSSE(w, "message_stop", map[string]any{"type": "message_stop"})

		// Build response for billing/logging
		var resp *gateway.ChatResponse
		if accumulatedUsage != nil {
			resp = &gateway.ChatResponse{Usage: accumulatedUsage}
		}

		statusCode := http.StatusOK
		if streamErr != nil {
			statusCode = http.StatusBadGateway
		}

		cost := calculateCost(route, resp)
		if cost > 0 {
			debitCtx, debitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer debitCancel()
			if _, err := h.billing.DebitUsage(debitCtx, tenantID, cost, ""); err != nil {
				slog.Error("failed to debit usage", "tenant_id", tenantID, "cost", cost, "error", err)
			}
		}

		h.usage.LogRequest(tenantID, route, requestedModel, resp, statusCode, duration, true, streamErr)
	})

	return nil
}

// ============================================================================
// Conversion helpers
// ============================================================================

// anthropicToChatRequest converts an Anthropic Messages request to an OpenAI ChatRequest.
func anthropicToChatRequest(req *anthropicMessagesRequest) gateway.ChatRequest {
	cr := gateway.ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Stop:        req.StopSequences,
	}

	maxTokens := req.MaxTokens
	cr.MaxTokens = &maxTokens

	// System message
	if req.System != nil {
		sysContent := ""
		switch v := req.System.(type) {
		case string:
			sysContent = v
		case []any:
			var parts []string
			for _, part := range v {
				if m, ok := part.(map[string]any); ok {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			sysContent = strings.Join(parts, "\n")
		}
		if sysContent != "" {
			cr.Messages = append(cr.Messages, gateway.Message{Role: "system", Content: sysContent})
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		m := gateway.Message{Role: msg.Role}

		switch content := msg.Content.(type) {
		case string:
			m.Content = content
		case []any:
			// Handle Anthropic content blocks
			var toolCalls []gateway.ToolCall
			var textParts []string
			var toolResults []map[string]any

			for _, block := range content {
				bm, ok := block.(map[string]any)
				if !ok {
					continue
				}
				blockType, _ := bm["type"].(string)

				switch blockType {
				case "text":
					if text, ok := bm["text"].(string); ok {
						textParts = append(textParts, text)
					}
				case "tool_use":
					args, _ := json.Marshal(bm["input"])
					toolCalls = append(toolCalls, gateway.ToolCall{
						ID:   bm["id"].(string),
						Type: "function",
						Function: gateway.ToolCallFunction{
							Name:      bm["name"].(string),
							Arguments: string(args),
						},
					})
				case "tool_result":
					toolResults = append(toolResults, bm)
				case "image":
					// Pass through as-is for vision models
					m.Content = content
				}
			}

			// If we have tool_results, split into separate messages
			if len(toolResults) > 0 {
				// This message should be split into individual tool messages
				// We handle the first one here; the caller's loop adds it
				for i, tr := range toolResults {
					toolMsg := gateway.Message{
						Role:       "tool",
						ToolCallID: tr["tool_use_id"].(string),
					}
					if trContent, ok := tr["content"].(string); ok {
						toolMsg.Content = trContent
					} else {
						contentJSON, _ := json.Marshal(tr["content"])
						toolMsg.Content = string(contentJSON)
					}
					if i == 0 {
						m = toolMsg
					} else {
						cr.Messages = append(cr.Messages, toolMsg)
					}
				}
				cr.Messages = append(cr.Messages, m)
				continue
			}

			if len(toolCalls) > 0 {
				m.ToolCalls = toolCalls
				if len(textParts) > 0 {
					m.Content = strings.Join(textParts, "\n")
				}
			} else if len(textParts) > 0 {
				m.Content = strings.Join(textParts, "\n")
			}
		default:
			m.Content = content
		}

		cr.Messages = append(cr.Messages, m)
	}

	// Convert tools
	for _, tool := range req.Tools {
		cr.Tools = append(cr.Tools, gateway.Tool{
			Type: "function",
			Function: gateway.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	// Convert tool_choice
	if req.ToolChoice != nil {
		switch v := req.ToolChoice.(type) {
		case map[string]any:
			if t, ok := v["type"].(string); ok {
				switch t {
				case "auto":
					cr.ToolChoice = "auto"
				case "any":
					cr.ToolChoice = "required"
				case "tool":
					if name, ok := v["name"].(string); ok {
						cr.ToolChoice = map[string]any{
							"type":     "function",
							"function": map[string]string{"name": name},
						}
					}
				}
			}
		}
	}

	return cr
}

// chatResponseToAnthropic converts an OpenAI ChatResponse to Anthropic Messages format.
func chatResponseToAnthropic(resp *gateway.ChatResponse, model string) *anthropicMsgResponse {
	ar := &anthropicMsgResponse{
		ID:    strings.Replace(resp.ID, "chatcmpl-", "msg_", 1),
		Type:  "message",
		Role:  "assistant",
		Model: model,
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message != nil {
			// Text content
			if choice.Message.Content != nil {
				if text, ok := choice.Message.Content.(string); ok && text != "" {
					ar.Content = append(ar.Content, anthropicContentBlock{
						Type: "text",
						Text: text,
					})
				}
			}

			// Tool calls
			for _, tc := range choice.Message.ToolCalls {
				var input any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				ar.Content = append(ar.Content, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
		}

		ar.StopReason = mapFinishReasonToAnthropicPtr(choice.FinishReason)
	}

	if ar.Content == nil {
		ar.Content = []anthropicContentBlock{}
	}

	if resp.Usage != nil {
		ar.Usage = &anthropicMsgUsage{
			InputTokens:              resp.Usage.PromptTokens,
			OutputTokens:             resp.Usage.CompletionTokens,
			CacheReadInputTokens:     resp.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: resp.Usage.CacheCreationInputToken,
		}
	}

	return ar
}

func mapFinishReasonToAnthropic(reason *string) string {
	if reason == nil {
		return "end_turn"
	}
	switch *reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

func mapFinishReasonToAnthropicPtr(reason *string) *string {
	r := mapFinishReasonToAnthropic(reason)
	return &r
}

func writeAnthropicSSE(w *bufio.Writer, eventType string, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	w.Flush()
}

// anthropicError returns an error in Anthropic's error envelope format.
func anthropicError(c *fiber.Ctx, status int, errType, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"type": "error",
		"error": fiber.Map{
			"type":    errType,
			"message": message,
		},
	})
}
