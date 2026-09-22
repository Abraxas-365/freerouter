package gatewayhttp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/gateway"
	"github.com/gofiber/fiber/v2"
)

// ============================================================================
// Anthropic Messages API types (client-facing)
// ============================================================================

type anthropicMessagesRequest struct {
	Model         string                     `json:"model"`
	Messages      []anthropicMsg             `json:"messages"`
	System        any                        `json:"system,omitempty"` // string or []block
	MaxTokens     int                        `json:"max_tokens"`
	Temperature   *float64                   `json:"temperature,omitempty"`
	TopP          *float64                   `json:"top_p,omitempty"`
	TopK          *int                       `json:"top_k,omitempty"`
	Stream        bool                       `json:"stream,omitempty"`
	StopSequences any                        `json:"stop_sequences,omitempty"`
	Tools         []anthropicToolDef         `json:"tools,omitempty"`
	ToolChoice    any                        `json:"tool_choice,omitempty"`
	Metadata      any                        `json:"metadata,omitempty"`
	Thinking      *anthropicMsgThinking      `json:"thinking,omitempty"`
	OutputConfig  *anthropicMsgOutputConfig  `json:"output_config,omitempty"`
}

type anthropicMsgThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
	Display      string `json:"display,omitempty"`
}

type anthropicMsgOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type anthropicMsg struct {
	Role         string                 `json:"role"`
	Content      any                    `json:"content"` // string or []contentBlock
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
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
	Type      string `json:"type"` // "text", "tool_use", or "thinking"
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
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
func (h *Handler) AnthropicMessages(c *fiber.Ctx) error {
	if h.metrics != nil {
		h.metrics.InFlightRequests.WithLabelValues(string(gateway.ProtocolAnthropic)).Inc()
		defer h.metrics.InFlightRequests.WithLabelValues(string(gateway.ProtocolAnthropic)).Dec()
	}

	// Rate limit check
	subjectID, release, err := h.checkRateLimit(c)
	if err != nil {
		return anthropicErrorFromErrx(c, err)
	}
	if release != nil {
		defer release()
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

	// Guardrails: check messages before routing
	if h.guardrails != nil {
		texts := extractAnthropicTexts(req.Messages)
		result, err := h.guardrails.CheckMessages(c.Context(), texts, req.Model)
		if err != nil {
			slog.Error("guardrails check failed", "error", err)
		} else if result.Blocked {
			return anthropicError(c, http.StatusBadRequest, "invalid_request_error", "Request blocked by content policy")
		}
	}

	// Convert to internal ChatRequest
	chatReq := anthropicToChatRequest(&req)

	// Resolve all candidate routes for retry/fallback, ordered by the
	// subject's configured routing strategy.
	strategy := h.resolveStrategy(c.Context(), subjectID)
	routes, err := h.router.ResolveAll(c.Context(), chatReq.Model, strategy)
	if err != nil {
		return anthropicErrorFromErrx(c, err)
	}
	if len(routes) == 0 {
		return anthropicError(c, http.StatusNotFound, "invalid_request_error", "no available route for model: "+chatReq.Model)
	}

	if req.Stream {
		return h.handleAnthropicStreamWithRetry(c, routes, &chatReq, requestedModel)
	}

	// Check response cache
	var cacheKey string
	if h.cache != nil {
		cacheKey = gateway.GenerateKey(subjectID, &chatReq)
		if cached := h.cache.Get(c.Context(), cacheKey); cached != nil {
			c.Set("X-Cache", "HIT")
			if h.metrics != nil {
				h.metrics.ObserveCacheHit()
			}
			anthropicResp := chatResponseToAnthropic(cached, requestedModel)
			return c.JSON(anthropicResp)
		}
		c.Set("X-Cache", "MISS")
		if h.metrics != nil {
			h.metrics.ObserveCacheMiss()
		}
	}

	return h.handleAnthropicNonStreamWithRetry(c, routes, &chatReq, requestedModel, cacheKey)
}

func (h *Handler) handleAnthropicNonStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, chatReq *gateway.ChatRequest, requestedModel string, cacheKey string) error {
	var lastErr error
	var lastStatus int

	for attempt, route := range routes {
		if attempt > 0 {
			delay := gateway.RetryDelay(attempt - 1)
			time.Sleep(delay)
		}

		chatReq.Model = route.ExternalID
		body, _ := json.Marshal(chatReq)

		start := time.Now()
		resp, status, err := h.upstream.Call(c.Context(), route, body)
		latency := time.Since(start)

		if err == nil {
			h.healthTracker.ReportSuccessWithLatency(route.KeyID, latency)

			h.logUsage(route, requestedModel, resp, http.StatusOK, latency, false, nil)

			if cacheKey != "" && h.cache != nil {
				h.cache.Set(c.Context(), cacheKey, resp)
			}

			if h.metrics != nil {
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolAnthropic, gateway.StatusOK, latency)
				if resp.Usage != nil {
					h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			anthropicResp := chatResponseToAnthropic(resp, requestedModel)
			return c.JSON(anthropicResp)
		}

		lastErr = err
		lastStatus = status

		h.logUsage(route, requestedModel, nil, status, latency, false, err)

		if gateway.IsAuthError(status) {
			if h.tokenRefresher != nil {
				cred, refreshErr := h.tokenRefresher.ForceRefresh(c.Context(), route.KeyID)
				if refreshErr == nil && cred.OAuth != nil {
					route.Token = cred.OAuth.AccessToken
					retryStart := time.Now()
					retryResp, retryStatus, retryErr := h.upstream.Call(c.Context(), route, body)
					retryLatency := time.Since(retryStart)
					if retryErr == nil {
						h.healthTracker.ReportSuccessWithLatency(route.KeyID, retryLatency)
						h.logUsage(route, requestedModel, retryResp, http.StatusOK, retryLatency, false, nil)
						if cacheKey != "" && h.cache != nil {
							h.cache.Set(c.Context(), cacheKey, retryResp)
						}
						if h.metrics != nil {
							h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolAnthropic, gateway.StatusOK, retryLatency)
							if retryResp.Usage != nil {
								h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), retryResp.Usage.PromptTokens, retryResp.Usage.CompletionTokens)
							}
						}
						anthropicResp := chatResponseToAnthropic(retryResp, requestedModel)
						return c.JSON(anthropicResp)
					}
					lastErr = retryErr
					lastStatus = retryStatus
					h.logUsage(route, requestedModel, nil, retryStatus, retryLatency, false, retryErr)
					if h.metrics != nil {
						h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", retryStatus))
					}
				}
			}
			h.healthTracker.ReportError(route.KeyID, status)
			h.fireKeyHealthWebhook(route, status)
			if h.metrics != nil {
				h.metrics.ObserveRetry(route.ProviderID.String(), "auth_error")
				h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
			}
			continue
		}

		if gateway.IsRetryable(status) {
			h.healthTracker.ReportError(route.KeyID, status)
			h.fireKeyHealthWebhook(route, status)
			if h.metrics != nil {
				h.metrics.ObserveRetry(route.ProviderID.String(), fmt.Sprintf("http_%d", status))
				h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
			}
			continue
		}

		// Non-retryable error
		h.healthTracker.ReportError(route.KeyID, status)
		h.fireKeyHealthWebhook(route, status)
		if h.metrics != nil {
			h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolAnthropic, gateway.StatusError, latency)
			h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
		}
		break
	}

	if lastErr != nil {
		return anthropicError(c, http.StatusBadGateway, "api_error", lastErr.Error())
	}
	return anthropicError(c, http.StatusBadGateway, "api_error", fmt.Sprintf("all routes exhausted, last status: %d", lastStatus))
}

func (h *Handler) handleAnthropicStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, chatReq *gateway.ChatRequest, requestedModel string) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		streamCtx, streamCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer streamCancel()

		var lastErr error
		var lastStatus int

		for attempt, route := range routes {
			if attempt > 0 {
				time.Sleep(gateway.RetryDelay(attempt - 1))
			}

			chatReq.Model = route.ExternalID
			body, _ := json.Marshal(chatReq)

			start := time.Now()
			var accumulatedUsage *gateway.Usage
			var lastFinishReason *string
			headersSent := false
			blockIndex := 0

			// Track which block types are currently open so we can
			// close them before opening a new one of a different type.
			const (
				blockNone     = 0
				blockThinking = 1
				blockText     = 2
				blockToolUse  = 3
			)
			currentBlockType := blockNone

			// closeCurrentBlock closes the open content block if any.
			closeCurrentBlock := func() {
				if currentBlockType != blockNone {
					writeAnthropicSSE(w, "content_block_stop", map[string]any{
						"type": "content_block_stop", "index": blockIndex,
					})
					blockIndex++
					currentBlockType = blockNone
				}
			}

			upstreamStatus, streamErr := h.upstream.Stream(streamCtx, route, body, func(chunk []byte) error {
				data := extractSSEData(chunk)
				if data == "" {
					return nil
				}
				if data == "[DONE]" {
					return nil
				}

				if !headersSent {
					headersSent = true
					msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
					writeAnthropicSSE(w, "message_start", map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id": msgID, "type": "message", "role": "assistant",
							"model": requestedModel, "content": []any{},
							"usage": map[string]int{"input_tokens": 0, "output_tokens": 0},
						},
					})
				}

				var streamChunk gateway.ChatStreamChunk
				if err := json.Unmarshal([]byte(data), &streamChunk); err != nil {
					return nil
				}
				if streamChunk.Usage != nil {
					accumulatedUsage = streamChunk.Usage
				}
				for _, choice := range streamChunk.Choices {
					if choice.FinishReason != nil {
						lastFinishReason = choice.FinishReason
					}
					if choice.Delta == nil {
						continue
					}

					// Thinking/reasoning deltas → Anthropic thinking content block.
					if choice.Delta.Reasoning != "" {
						if currentBlockType != blockThinking {
							closeCurrentBlock()
							currentBlockType = blockThinking
							writeAnthropicSSE(w, "content_block_start", map[string]any{
								"type": "content_block_start", "index": blockIndex,
								"content_block": map[string]string{"type": "thinking", "thinking": ""},
							})
						}
						writeAnthropicSSE(w, "content_block_delta", map[string]any{
							"type": "content_block_delta", "index": blockIndex,
							"delta": map[string]string{"type": "thinking_delta", "thinking": choice.Delta.Reasoning},
						})
					}

					// Signature delta → emit inside thinking block (open one if needed).
					if choice.Delta.Signature != "" {
						if currentBlockType != blockThinking {
							closeCurrentBlock()
							currentBlockType = blockThinking
							writeAnthropicSSE(w, "content_block_start", map[string]any{
								"type": "content_block_start", "index": blockIndex,
								"content_block": map[string]string{"type": "thinking", "thinking": ""},
							})
						}
						writeAnthropicSSE(w, "content_block_delta", map[string]any{
							"type": "content_block_delta", "index": blockIndex,
							"delta": map[string]string{"type": "signature_delta", "signature": choice.Delta.Signature},
						})
					}

					// Text deltas → Anthropic text content block.
					if choice.Delta.Content != nil {
						if text, ok := choice.Delta.Content.(string); ok && text != "" {
							if currentBlockType != blockText {
								closeCurrentBlock()
								currentBlockType = blockText
								writeAnthropicSSE(w, "content_block_start", map[string]any{
									"type": "content_block_start", "index": blockIndex,
									"content_block": map[string]string{"type": "text", "text": ""},
								})
							}
							writeAnthropicSSE(w, "content_block_delta", map[string]any{
								"type": "content_block_delta", "index": blockIndex,
								"delta": map[string]string{"type": "text_delta", "text": text},
							})
						}
					}

					// Tool call deltas → Anthropic tool_use content blocks.
					for _, tc := range choice.Delta.ToolCalls {
						if tc.ID != "" {
							// New tool call: close any open block and start a tool_use block.
							closeCurrentBlock()
							currentBlockType = blockToolUse
							writeAnthropicSSE(w, "content_block_start", map[string]any{
								"type": "content_block_start", "index": blockIndex,
								"content_block": map[string]any{
									"type":  "tool_use",
									"id":    tc.ID,
									"name":  tc.Function.Name,
									"input": map[string]any{},
								},
							})
						}
						if tc.Function.Arguments != "" {
							writeAnthropicSSE(w, "content_block_delta", map[string]any{
								"type": "content_block_delta", "index": blockIndex,
								"delta": map[string]string{
									"type":         "input_json_delta",
									"partial_json": tc.Function.Arguments,
								},
							})
						}
					}
				}
				return nil
			})

			duration := time.Since(start)

			if streamErr != nil {
				h.healthTracker.ReportError(route.KeyID, upstreamStatus)
				h.fireKeyHealthWebhook(route, upstreamStatus)

				if headersSent {
					// Close any open block, ensuring at least one exists.
					if currentBlockType == blockNone {
						writeAnthropicSSE(w, "content_block_start", map[string]any{
							"type": "content_block_start", "index": blockIndex,
							"content_block": map[string]string{"type": "text", "text": ""},
						})
						currentBlockType = blockText
					}
					closeCurrentBlock()
					h.finishAnthropicStream(w, lastFinishReason, accumulatedUsage)

					var resp *gateway.ChatResponse
					if accumulatedUsage != nil {
						resp = &gateway.ChatResponse{Usage: accumulatedUsage}
					}
					h.logUsage(route, requestedModel, resp, http.StatusBadGateway, duration, true, streamErr)
					if h.metrics != nil {
						h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolAnthropic, gateway.StatusError, duration)
					}
					return
				}

				lastErr = streamErr
				lastStatus = upstreamStatus

				if gateway.IsAuthError(upstreamStatus) {
					if h.metrics != nil {
						h.metrics.ObserveRetry(route.ProviderID.String(), "auth_error")
						h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", upstreamStatus))
					}
					continue
				}
				if gateway.IsRetryable(upstreamStatus) && attempt < len(routes)-1 {
					if h.metrics != nil {
						h.metrics.ObserveRetry(route.ProviderID.String(), fmt.Sprintf("http_%d", upstreamStatus))
						h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", upstreamStatus))
					}
					continue
				}

				// Non-retryable
				errJSON, _ := json.Marshal(fiber.Map{"type": "error", "error": fiber.Map{"type": "api_error", "message": streamErr.Error()}})
				_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", errJSON)
				_ = w.Flush()
				h.logUsage(route, requestedModel, nil, upstreamStatus, duration, true, streamErr)
				if h.metrics != nil {
					h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolAnthropic, gateway.StatusError, duration)
					h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", upstreamStatus))
				}
				return
			}

			// Success
			h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)
			// Close any open block and ensure at least one was started.
			closeCurrentBlock()
			if blockIndex == 0 {
				writeAnthropicSSE(w, "content_block_start", map[string]any{
					"type": "content_block_start", "index": 0,
					"content_block": map[string]string{"type": "text", "text": ""},
				})
				writeAnthropicSSE(w, "content_block_stop", map[string]any{
					"type": "content_block_stop", "index": 0,
				})
				blockIndex = 1
			}
			h.finishAnthropicStream(w, lastFinishReason, accumulatedUsage)

			var resp *gateway.ChatResponse
			if accumulatedUsage != nil {
				resp = &gateway.ChatResponse{Usage: accumulatedUsage}
			}

			if h.metrics != nil {
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolAnthropic, gateway.StatusOK, duration)
				if resp != nil && resp.Usage != nil {
					h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			h.logUsage(route, requestedModel, resp, http.StatusOK, duration, true, nil)
			return
		}

		// All attempts exhausted
		errJSON, _ := json.Marshal(fiber.Map{"type": "error", "error": fiber.Map{"type": "api_error", "message": fmt.Sprintf("all stream attempts failed: %v", lastErr)}})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", errJSON)
		_ = w.Flush()
		if len(routes) > 0 {
			h.logUsage(routes[0], requestedModel, nil, lastStatus, 0, true, lastErr)
		}
	})

	return nil
}

// finishAnthropicStream writes the closing Anthropic SSE event sequence.
// All content blocks must already be closed before calling this.
func (h *Handler) finishAnthropicStream(w *bufio.Writer, lastFinishReason *string, usage *gateway.Usage) {
	stopReason := mapFinishReasonToAnthropic(lastFinishReason)
	deltaEvt := map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stopReason}}
	if usage != nil {
		deltaEvt["usage"] = map[string]int{"output_tokens": usage.CompletionTokens}
	}
	writeAnthropicSSE(w, "message_delta", deltaEvt)
	writeAnthropicSSE(w, "message_stop", map[string]any{"type": "message_stop"})
}

// extractSSEData strips the "data: " prefix from a raw SSE chunk emitted by
// the OpenAI-compatible upstream translator (see gateway.Upstream.Stream).
func extractSSEData(chunk []byte) string {
	s := strings.TrimSpace(string(chunk))
	s = strings.TrimPrefix(s, "data: ")
	return strings.TrimSpace(s)
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
		TopK:        req.TopK,
		Stream:      req.Stream,
		Stop:        req.StopSequences,
	}

	maxTokens := req.MaxTokens
	cr.MaxTokens = &maxTokens

	// Thinking passthrough
	if req.Thinking != nil {
		cr.Thinking = &gateway.ThinkingConfig{
			Type:         req.Thinking.Type,
			BudgetTokens: req.Thinking.BudgetTokens,
			Display:      req.Thinking.Display,
		}
	}

	// Output config passthrough (effort for adaptive thinking)
	if req.OutputConfig != nil {
		cr.OutputConfig = &gateway.OutputConfig{Effort: req.OutputConfig.Effort}
	}

	// System message — preserve cache_control from block arrays.
	if req.System != nil {
		sysMsg := gateway.Message{Role: "system"}
		switch v := req.System.(type) {
		case string:
			sysMsg.Content = v
		case []any:
			var parts []string
			for _, part := range v {
				if m, ok := part.(map[string]any); ok {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
					// Capture cache_control from the last system block.
					if cc, ok := m["cache_control"].(map[string]any); ok {
						ccTyped := make(map[string]interface{}, len(cc))
						for k, val := range cc {
							ccTyped[k] = val
						}
						sysMsg.CacheControl = ccTyped
					}
				}
			}
			sysMsg.Content = strings.Join(parts, "\n")
		}
		if sysContent, ok := sysMsg.Content.(string); ok && sysContent != "" {
			cr.Messages = append(cr.Messages, sysMsg)
		}
	}

	// Convert messages
	for _, msg := range req.Messages {
		m := gateway.Message{
			Role:         msg.Role,
			CacheControl: msg.CacheControl,
		}

		switch content := msg.Content.(type) {
		case string:
			m.Content = content
		case []any:
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
					// Capture cache_control from text blocks.
					if cc, ok := bm["cache_control"].(map[string]any); ok && m.CacheControl == nil {
						ccTyped := make(map[string]interface{}, len(cc))
						for k, val := range cc {
							ccTyped[k] = val
						}
						m.CacheControl = ccTyped
					}
				case "thinking":
					if thinking, ok := bm["thinking"].(string); ok {
						m.Reasoning = thinking
					}
				case "tool_use":
					args, _ := json.Marshal(bm["input"])
					id, _ := bm["id"].(string)
					name, _ := bm["name"].(string)
					toolCalls = append(toolCalls, gateway.ToolCall{
						ID:   id,
						Type: "function",
						Function: gateway.ToolCallFunction{
							Name:      name,
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
				for i, tr := range toolResults {
					toolCallID, _ := tr["tool_use_id"].(string)
					toolMsg := gateway.Message{
						Role:       "tool",
						ToolCallID: toolCallID,
					}
					if trContent, ok := tr["content"].(string); ok {
						toolMsg.Content = trContent
					} else {
						contentJSON, _ := json.Marshal(tr["content"])
						toolMsg.Content = string(contentJSON)
					}
					// Preserve cache_control on tool results.
					if cc, ok := tr["cache_control"].(map[string]any); ok {
						ccTyped := make(map[string]interface{}, len(cc))
						for k, val := range cc {
							ccTyped[k] = val
						}
						toolMsg.CacheControl = ccTyped
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
		if v, ok := req.ToolChoice.(map[string]any); ok {
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
			// Emit thinking block before text (Anthropic convention).
			if choice.Message.Reasoning != "" {
				ar.Content = append(ar.Content, anthropicContentBlock{
					Type:     "thinking",
					Thinking: choice.Message.Reasoning,
				})
			}

			if choice.Message.Content != nil {
				if text, ok := choice.Message.Content.(string); ok && text != "" {
					ar.Content = append(ar.Content, anthropicContentBlock{
						Type: "text",
						Text: text,
					})
				}
			}

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
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	_ = w.Flush()
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

// anthropicErrorFromErrx converts an internal error (typically *errx.Error)
// into an Anthropic-shaped error response, bypassing the default JSON error
// middleware so /v1/messages always returns Anthropic's envelope.
func anthropicErrorFromErrx(c *fiber.Ctx, err error) error {
	status, message := errxStatusAndMessage(err)
	return anthropicError(c, status, "api_error", message)
}

// errxStatusAndMessage extracts the HTTP status and message from an error,
// unwrapping *errx.Error when possible and falling back to 500/generic text.
func errxStatusAndMessage(err error) (int, string) {
	var ex *errx.Error
	if errors.As(err, &ex) {
		return ex.HTTPStatus, ex.Message
	}
	return http.StatusInternalServerError, err.Error()
}

// extractAnthropicTexts extracts text content from Anthropic messages.
func extractAnthropicTexts(messages []anthropicMsg) []string {
	texts := make([]string, 0, len(messages))
	for _, m := range messages {
		if v, ok := m.Content.(string); ok {
			texts = append(texts, v)
		}
	}
	return texts
}
