package gatewayapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

// ============================================================================
// OpenAI Responses API types
// ============================================================================

type responsesRequest struct {
	Model           string         `json:"model"`
	Input           any            `json:"input"` // string or []inputItem
	Instructions    string         `json:"instructions,omitempty"`
	MaxOutputTokens *int           `json:"max_output_tokens,omitempty"`
	Temperature     *float64       `json:"temperature,omitempty"`
	TopP            *float64       `json:"top_p,omitempty"`
	Stream          bool           `json:"stream,omitempty"`
	Tools           []any          `json:"tools,omitempty"`
	ToolChoice      any            `json:"tool_choice,omitempty"`
	Reasoning       *respReasoning `json:"reasoning,omitempty"`
	Text            *respText      `json:"text,omitempty"`
}

type respReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type respText struct {
	Format any `json:"format,omitempty"`
}

type responsesResponse struct {
	ID        string       `json:"id"`
	Object    string       `json:"object"` // "response"
	CreatedAt int64        `json:"created_at"`
	Status    string       `json:"status"` // "completed", "incomplete", "failed"
	Output    []respOutput `json:"output"`
	Model     string       `json:"model"`
	Usage     *respUsage   `json:"usage,omitempty"`
}

type respOutput struct {
	Type    string        `json:"type"` // "message", "function_call"
	ID      string        `json:"id,omitempty"`
	Role    string        `json:"role,omitempty"` // for message
	Status  string        `json:"status"`
	Content []respContent `json:"content,omitempty"`   // for message
	CallID  string        `json:"call_id,omitempty"`   // for function_call
	Name    string        `json:"name,omitempty"`      // for function_call
	Args    string        `json:"arguments,omitempty"` // for function_call
}

type respContent struct {
	Type string `json:"type"` // "output_text"
	Text string `json:"text"`
}

type respUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ============================================================================
// Handler
// ============================================================================

// Responses handles POST /v1/responses (OpenAI Responses API).
func (h *GatewayHandlers) Responses(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	// Rate limit check
	if err := h.checkRateLimit(c, authCtx.TenantID); err != nil {
		return err
	}
	defer h.rateLimiter.Release(c.Context(), authCtx.TenantID.String())

	// Spending limit check
	if err := h.checkSpendingLimit(c, authCtx.TenantID); err != nil {
		return err
	}
	if err := h.checkWalletBalance(c, authCtx); err != nil {
		return err
	}

	var req responsesRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	requestedModel := req.Model

	// Per-key model restriction
	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}

	// Guardrails: check input before routing
	if h.guardrails != nil {
		texts := extractResponsesTexts(&req)
		result, err := h.guardrails.CheckMessages(c.Context(), authCtx.TenantID, texts, req.Model)
		if err != nil {
			slog.Error("guardrails check failed", "error", err)
		} else if result.Blocked {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fiber.Map{
					"message":    "Request blocked by content policy",
					"type":       "guardrail_violation",
					"code":       "content_policy_violation",
					"violations": result.Violations,
				},
			})
		}
	}

	// Convert to internal ChatRequest
	chatReq := responsesToChatRequest(&req)

	// Resolve all candidate routes for retry/fallback
	tenantID := &authCtx.TenantID
	routes, err := h.router.ResolveAll(c.Context(), chatReq.Model, tenantID, chatReq.MaxTokens)
	if err != nil {
		return err
	}

	if req.Stream {
		return h.handleResponsesStreamWithRetry(c, routes, &chatReq, requestedModel, authCtx.TenantID, authCtx.WalletID)
	}

	// Check response cache
	cacheKey := gateway.GenerateKey(authCtx.TenantID.String(), &chatReq)
	if h.cache != nil {
		if cached := h.cache.Get(c.Context(), cacheKey); cached != nil {
			c.Set("X-Cache", "HIT")
			respAPI := chatResponseToResponses(cached, requestedModel)
			return c.JSON(respAPI)
		}
	}

	return h.handleResponsesNonStreamWithRetry(c, routes, &chatReq, requestedModel, authCtx.TenantID, authCtx.WalletID, cacheKey)
}

func (h *GatewayHandlers) handleResponsesNonStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, chatReq *gateway.ChatRequest, requestedModel string, tenantID kernel.TenantID, walletID *kernel.WalletID, cacheKey string) error {
	maxAttempts := gateway.MaxRetries + 1
	if maxAttempts > len(routes) {
		maxAttempts = len(routes)
	}

	var lastErr error
	var lastStatus int

	for attempt := 0; attempt < maxAttempts; attempt++ {
		route := routes[attempt]
		chatReq.Model = route.ExternalID
		body, _ := json.Marshal(chatReq)

		start := time.Now()
		resp, statusCode, err := h.upstream.Call(c.Context(), route, body)
		duration := time.Since(start)

		if err != nil {
			h.healthTracker.ReportError(route.KeyID, statusCode)
			lastErr = err
			lastStatus = statusCode

			if gateway.IsAuthError(statusCode) {
				continue
			}
			if gateway.IsRetryable(statusCode) && attempt < maxAttempts-1 {
				time.Sleep(gateway.RetryDelay(attempt))
				continue
			}
			h.usage.LogRequest(tenantID, route, requestedModel, nil, statusCode, duration, false, err, nil)
			return err
		}

		h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

		cost := calculateCost(route, resp)
		h.debitUsage(c.Context(), tenantID, walletID, cost)

		h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, false, nil, buildRequestContent(c, chatReq.Messages, resp, body))
		h.fireRequestWebhook(tenantID, route, requestedModel, resp, http.StatusOK, duration, nil)

		if h.cache != nil {
			c.Set("X-Cache", "MISS")
			h.cache.Set(c.Context(), cacheKey, resp)
		}

		respAPI := chatResponseToResponses(resp, requestedModel)
		return c.JSON(respAPI)
	}

	h.usage.LogRequest(tenantID, routes[0], requestedModel, nil, lastStatus, 0, false, lastErr, nil)
	h.fireRequestWebhook(tenantID, routes[0], requestedModel, nil, lastStatus, 0, lastErr)
	return lastErr
}

func (h *GatewayHandlers) handleResponsesStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, chatReq *gateway.ChatRequest, requestedModel string, tenantID kernel.TenantID, walletID *kernel.WalletID) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	// Capture debug mode + raw body before entering the async stream writer;
	// the fiber.Ctx is not safe to use once the handler returns.
	debugMode := isDebugMode(c)
	rawBody := append([]byte(nil), c.Body()...)

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		streamCtx, streamCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer streamCancel()

		maxAttempts := gateway.MaxRetries + 1
		if maxAttempts > len(routes) {
			maxAttempts = len(routes)
		}

		var lastErr error
		var lastStatus int

		for attempt := 0; attempt < maxAttempts; attempt++ {
			route := routes[attempt]
			chatReq.Model = route.ExternalID
			body, _ := json.Marshal(chatReq)

			start := time.Now()
			var accumulatedUsage *gateway.Usage
			var lastFinishReason *string
			headersSent := false
			respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
			msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
			seqNum := 0

			upstreamStatus, streamErr := h.upstream.Stream(streamCtx, route, body, func(chunk []byte) error {
				// First chunk: write framing events
				if !headersSent {
					headersSent = true
					writeResponsesSSE(w, "response.created", map[string]any{
						"response": map[string]any{
							"id": respID, "object": "response", "created_at": time.Now().Unix(),
							"status": "in_progress", "output": []any{}, "model": requestedModel,
						},
						"sequence_number": seqNum,
					})
					seqNum++
					writeResponsesSSE(w, "response.output_item.added", map[string]any{
						"output_index": 0,
						"item": map[string]any{
							"type": "message", "id": msgID, "role": "assistant",
							"status": "in_progress", "content": []any{},
						},
						"sequence_number": seqNum,
					})
					seqNum++
					writeResponsesSSE(w, "response.content_part.added", map[string]any{
						"output_index": 0, "content_index": 0,
						"part":            map[string]string{"type": "output_text", "text": ""},
						"sequence_number": seqNum,
					})
					seqNum++
				}

				var streamChunk gateway.ChatStreamChunk
				if err := json.Unmarshal(chunk, &streamChunk); err != nil {
					return nil
				}
				if streamChunk.Usage != nil {
					accumulatedUsage = streamChunk.Usage
				}
				for _, choice := range streamChunk.Choices {
					if choice.FinishReason != nil {
						lastFinishReason = choice.FinishReason
					}
					if choice.Delta != nil && choice.Delta.Content != nil {
						if text, ok := choice.Delta.Content.(string); ok && text != "" {
							writeResponsesSSE(w, "response.output_text.delta", map[string]any{
								"output_index": 0, "content_index": 0,
								"delta": text, "sequence_number": seqNum,
							})
							seqNum++
						}
					}
				}
				return nil
			})

			duration := time.Since(start)

			if streamErr != nil {
				h.healthTracker.ReportError(route.KeyID, upstreamStatus)

				// Mid-stream failure: can't retry
				if headersSent {
					writeResponsesSSE(w, "response.content_part.done", map[string]any{
						"output_index": 0, "content_index": 0,
						"part":            map[string]string{"type": "output_text", "text": ""},
						"sequence_number": seqNum,
					})
					seqNum++
					writeResponsesSSE(w, "response.output_item.done", map[string]any{
						"output_index":    0,
						"item":            map[string]any{"type": "message", "id": msgID, "role": "assistant", "status": "incomplete"},
						"sequence_number": seqNum,
					})
					seqNum++
					writeResponsesSSE(w, "response.failed", map[string]any{
						"response": map[string]any{
							"id": respID, "object": "response", "created_at": time.Now().Unix(),
							"status": "failed", "model": requestedModel,
						},
						"sequence_number": seqNum,
					})

					var resp *gateway.ChatResponse
					if accumulatedUsage != nil {
						resp = &gateway.ChatResponse{Usage: accumulatedUsage}
					}
					cost := calculateCost(route, resp)
					dCtx, dc := context.WithTimeout(context.Background(), 5*time.Second)
					defer dc()
					h.debitUsage(dCtx, tenantID, walletID, cost)
					h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusBadGateway, duration, true, streamErr, nil)
					return
				}

				// Pre-stream error: can retry
				lastErr = streamErr
				lastStatus = upstreamStatus

				if gateway.IsAuthError(upstreamStatus) {
					if h.metrics != nil {
						h.metrics.ObserveRetry(route.ProviderID.String(), "auth_error")
					}
					continue
				}
				if gateway.IsRetryable(upstreamStatus) && attempt < maxAttempts-1 {
					if h.metrics != nil {
						h.metrics.ObserveRetry(route.ProviderID.String(), fmt.Sprintf("http_%d", upstreamStatus))
					}
					time.Sleep(gateway.RetryDelay(attempt))
					continue
				}

				// Non-retryable
				errEvt := map[string]any{
					"response": map[string]any{
						"id": respID, "object": "response", "created_at": time.Now().Unix(),
						"status": "failed", "model": requestedModel,
					},
					"sequence_number": 0,
				}
				writeResponsesSSE(w, "response.failed", errEvt)
				h.usage.LogRequest(tenantID, route, requestedModel, nil, upstreamStatus, duration, true, streamErr, nil)
				return
			}

			// Success
			h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

			writeResponsesSSE(w, "response.content_part.done", map[string]any{
				"output_index": 0, "content_index": 0,
				"part":            map[string]string{"type": "output_text", "text": ""},
				"sequence_number": seqNum,
			})
			seqNum++

			status := "completed"
			if lastFinishReason != nil && *lastFinishReason == "length" {
				status = "incomplete"
			}
			writeResponsesSSE(w, "response.output_item.done", map[string]any{
				"output_index":    0,
				"item":            map[string]any{"type": "message", "id": msgID, "role": "assistant", "status": status},
				"sequence_number": seqNum,
			})
			seqNum++

			completedResp := map[string]any{
				"id": respID, "object": "response", "created_at": time.Now().Unix(),
				"status": status, "model": requestedModel,
			}
			if accumulatedUsage != nil {
				completedResp["usage"] = map[string]int{
					"input_tokens":  accumulatedUsage.PromptTokens,
					"output_tokens": accumulatedUsage.CompletionTokens,
					"total_tokens":  accumulatedUsage.TotalTokens,
				}
			}
			writeResponsesSSE(w, "response.completed", map[string]any{
				"response": completedResp, "sequence_number": seqNum,
			})

			var resp *gateway.ChatResponse
			if accumulatedUsage != nil {
				resp = &gateway.ChatResponse{Usage: accumulatedUsage}
			}

			cost := calculateCost(route, resp)
			dCtx, dc := context.WithTimeout(context.Background(), 5*time.Second)
			defer dc()
			h.debitUsage(dCtx, tenantID, walletID, cost)

			if h.metrics != nil {
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "responses", "ok", duration)
				if resp != nil && resp.Usage != nil {
					h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, true, nil, buildStreamRequestContent(debugMode, rawBody, chatReq.Messages))
			h.fireRequestWebhook(tenantID, route, requestedModel, resp, http.StatusOK, duration, nil)
			return
		}

		// All attempts exhausted
		errEvt := map[string]any{
			"response": map[string]any{
				"id": fmt.Sprintf("resp_%d", time.Now().UnixNano()), "object": "response",
				"created_at": time.Now().Unix(), "status": "failed", "model": requestedModel,
			},
			"sequence_number": 0,
		}
		writeResponsesSSE(w, "response.failed", errEvt)
		h.usage.LogRequest(tenantID, routes[0], requestedModel, nil, lastStatus, 0, true, lastErr, nil)
		h.fireRequestWebhook(tenantID, routes[0], requestedModel, nil, lastStatus, 0, lastErr)
	})

	return nil
}

// ============================================================================
// Conversion helpers
// ============================================================================

// responsesToChatRequest converts an OpenAI Responses request to a ChatRequest.
func responsesToChatRequest(req *responsesRequest) gateway.ChatRequest {
	cr := gateway.ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		MaxTokens:   req.MaxOutputTokens,
	}

	// Instructions → system message
	if req.Instructions != "" {
		cr.Messages = append(cr.Messages, gateway.Message{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	// Reasoning
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		cr.ReasoningEffort = req.Reasoning.Effort
	}

	// Response format
	if req.Text != nil && req.Text.Format != nil {
		if fm, ok := req.Text.Format.(map[string]any); ok {
			if t, ok := fm["type"].(string); ok {
				cr.ResponseFormat = &gateway.ResponseFormat{Type: t}
				if t == "json_schema" {
					cr.ResponseFormat.JSONSchema = fm["schema"]
				}
			}
		}
	}

	// Convert input to messages
	switch v := req.Input.(type) {
	case string:
		cr.Messages = append(cr.Messages, gateway.Message{
			Role:    "user",
			Content: v,
		})
	case []any:
		for _, item := range v {
			im, ok := item.(map[string]any)
			if !ok {
				continue
			}

			itemType, _ := im["type"].(string)
			if itemType == "" {
				itemType = "message" // default
			}

			switch itemType {
			case "message":
				role, _ := im["role"].(string)
				if role == "" {
					role = "user"
				}
				msg := gateway.Message{Role: role}
				if content, ok := im["content"].(string); ok {
					msg.Content = content
				} else if content, ok := im["content"].([]any); ok {
					// Array of content parts
					var texts []string
					for _, part := range content {
						if pm, ok := part.(map[string]any); ok {
							if t, ok := pm["type"].(string); ok && t == "input_text" {
								if text, ok := pm["text"].(string); ok {
									texts = append(texts, text)
								}
							}
						}
					}
					if len(texts) > 0 {
						msg.Content = texts[0]
						if len(texts) > 1 {
							combined := ""
							for i, t := range texts {
								if i > 0 {
									combined += "\n"
								}
								combined += t
							}
							msg.Content = combined
						}
					} else {
						msg.Content = content
					}
				}
				cr.Messages = append(cr.Messages, msg)

			case "function_call":
				// Assistant message with tool call
				callID, _ := im["call_id"].(string)
				name, _ := im["name"].(string)
				args, _ := im["arguments"].(string)
				cr.Messages = append(cr.Messages, gateway.Message{
					Role: "assistant",
					ToolCalls: []gateway.ToolCall{{
						ID:   callID,
						Type: "function",
						Function: gateway.ToolCallFunction{
							Name:      name,
							Arguments: args,
						},
					}},
				})

			case "function_call_output":
				callID, _ := im["call_id"].(string)
				output, _ := im["output"].(string)
				cr.Messages = append(cr.Messages, gateway.Message{
					Role:       "tool",
					ToolCallID: callID,
					Content:    output,
				})
			}
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			tm, ok := tool.(map[string]any)
			if !ok {
				continue
			}
			toolType, _ := tm["type"].(string)
			if toolType != "function" {
				continue
			}
			name, _ := tm["name"].(string)
			desc, _ := tm["description"].(string)
			cr.Tools = append(cr.Tools, gateway.Tool{
				Type: "function",
				Function: gateway.ToolFunction{
					Name:        name,
					Description: desc,
					Parameters:  tm["parameters"],
				},
			})
		}
	}

	// Tool choice
	if req.ToolChoice != nil {
		cr.ToolChoice = req.ToolChoice
	}

	return cr
}

// chatResponseToResponses converts an OpenAI ChatResponse to Responses API format.
func chatResponseToResponses(resp *gateway.ChatResponse, model string) *responsesResponse {
	rr := &responsesResponse{
		ID:        fmt.Sprintf("resp_%d", time.Now().UnixNano()),
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Status:    "completed",
		Model:     model,
		Output:    []respOutput{},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]

		if choice.FinishReason != nil && *choice.FinishReason == "length" {
			rr.Status = "incomplete"
		}

		if choice.Message != nil {
			// Message output item
			msgOutput := respOutput{
				Type:    "message",
				ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
				Role:    "assistant",
				Status:  "completed",
				Content: []respContent{},
			}

			if choice.Message.Content != nil {
				if text, ok := choice.Message.Content.(string); ok && text != "" {
					msgOutput.Content = append(msgOutput.Content, respContent{
						Type: "output_text",
						Text: text,
					})
				}
			}

			rr.Output = append(rr.Output, msgOutput)

			// Function calls as separate output items
			for _, tc := range choice.Message.ToolCalls {
				rr.Output = append(rr.Output, respOutput{
					Type:   "function_call",
					ID:     fmt.Sprintf("fc_%s", tc.ID),
					CallID: tc.ID,
					Name:   tc.Function.Name,
					Args:   tc.Function.Arguments,
					Status: "completed",
				})
			}
		}
	}

	if resp.Usage != nil {
		rr.Usage = &respUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	return rr
}

func writeResponsesSSE(w *bufio.Writer, eventType string, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	w.Flush()
}

// extractResponsesTexts extracts text content from Responses API input.
func extractResponsesTexts(req *responsesRequest) []string {
	var texts []string
	if req.Instructions != "" {
		texts = append(texts, req.Instructions)
	}
	switch v := req.Input.(type) {
	case string:
		texts = append(texts, v)
	}
	return texts
}
