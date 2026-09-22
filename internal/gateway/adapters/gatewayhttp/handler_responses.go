package gatewayhttp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/gateway"
	"github.com/gofiber/fiber/v2"
)

// ============================================================================
// OpenAI Responses API types (client-facing)
// ============================================================================

type responsesRequest struct {
	Model              string         `json:"model"`
	Input              any            `json:"input"` // string or []inputItem
	Instructions       string         `json:"instructions,omitempty"`
	MaxOutputTokens    *int           `json:"max_output_tokens,omitempty"`
	Temperature        *float64       `json:"temperature,omitempty"`
	TopP               *float64       `json:"top_p,omitempty"`
	Stream             bool           `json:"stream,omitempty"`
	Tools              []any          `json:"tools,omitempty"`
	ToolChoice         any            `json:"tool_choice,omitempty"`
	Reasoning          *respReasoning `json:"reasoning,omitempty"`
	Text               *respText      `json:"text,omitempty"`
	Store              *bool          `json:"store,omitempty"`
	Truncation         string         `json:"truncation,omitempty"`
	Include            []string       `json:"include,omitempty"`
	PreviousResponseID string         `json:"previous_response_id,omitempty"`
}

type respReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"` // "auto", "concise", "detailed"
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
	Type    string                 `json:"type"` // "message", "function_call", "reasoning"
	ID      string                 `json:"id,omitempty"`
	Role    string                 `json:"role,omitempty"` // for message
	Status  string                 `json:"status"`
	Content []respContent          `json:"content,omitempty"`   // for message
	Summary []respReasoningSummary `json:"summary,omitempty"`   // for reasoning
	CallID  string                 `json:"call_id,omitempty"`   // for function_call
	Name    string                 `json:"name,omitempty"`      // for function_call
	Args    string                 `json:"arguments,omitempty"` // for function_call
}

type respContent struct {
	Type string `json:"type"` // "output_text"
	Text string `json:"text"`
}

type respReasoningSummary struct {
	Type string `json:"type"` // "summary_text"
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
func (h *Handler) Responses(c *fiber.Ctx) error {
	if h.metrics != nil {
		h.metrics.InFlightRequests.WithLabelValues(string(gateway.ProtocolResponses)).Inc()
		defer h.metrics.InFlightRequests.WithLabelValues(string(gateway.ProtocolResponses)).Dec()
	}

	// Rate limit check
	subjectID, release, err := h.checkRateLimit(c)
	if err != nil {
		return responsesErrorFromErrx(c, err)
	}
	if release != nil {
		defer release()
	}

	var req responsesRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return responsesError(c, http.StatusBadRequest, "invalid request body")
	}
	if req.Model == "" {
		return responsesError(c, http.StatusBadRequest, "model is required")
	}

	requestedModel := req.Model

	// Guardrails: check input before routing
	if h.guardrails != nil {
		texts := extractResponsesTexts(&req)
		result, err := h.guardrails.CheckMessages(c.Context(), texts, req.Model)
		if err != nil {
			slog.Error("guardrails check failed", "error", err)
		} else if result.Blocked {
			return responsesError(c, http.StatusBadRequest, "Request blocked by content policy")
		}
	}

	// Convert to internal ChatRequest
	chatReq := responsesToChatRequest(&req)

	// Resolve all candidate routes for retry/fallback, ordered by the
	// subject's configured routing strategy.
	strategy := h.resolveStrategy(c.Context(), subjectID)
	routes, err := h.router.ResolveAll(c.Context(), chatReq.Model, strategy)
	if err != nil {
		return responsesErrorFromErrx(c, err)
	}
	if len(routes) == 0 {
		return responsesError(c, http.StatusNotFound, "no available route for model: "+chatReq.Model)
	}

	if req.Stream {
		return h.handleResponsesStreamWithRetry(c, routes, &chatReq, requestedModel)
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
			respAPI := chatResponseToResponses(cached, requestedModel)
			return c.JSON(respAPI)
		}
		c.Set("X-Cache", "MISS")
		if h.metrics != nil {
			h.metrics.ObserveCacheMiss()
		}
	}

	return h.handleResponsesNonStreamWithRetry(c, routes, &chatReq, requestedModel, cacheKey)
}

func (h *Handler) handleResponsesNonStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, chatReq *gateway.ChatRequest, requestedModel string, cacheKey string) error {
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
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolResponses, gateway.StatusOK, latency)
				if resp.Usage != nil {
					h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			respAPI := chatResponseToResponses(resp, requestedModel)
			return c.JSON(respAPI)
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
							h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolResponses, gateway.StatusOK, retryLatency)
							if retryResp.Usage != nil {
								h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), retryResp.Usage.PromptTokens, retryResp.Usage.CompletionTokens)
							}
						}
						respAPI := chatResponseToResponses(retryResp, requestedModel)
						return c.JSON(respAPI)
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
			h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolResponses, gateway.StatusError, latency)
			h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", status))
		}
		break
	}

	if lastErr != nil {
		return responsesError(c, http.StatusBadGateway, lastErr.Error())
	}
	return responsesError(c, http.StatusBadGateway, fmt.Sprintf("all routes exhausted, last status: %d", lastStatus))
}

func (h *Handler) handleResponsesStreamWithRetry(c *fiber.Ctx, routes []*gateway.RouteResult, chatReq *gateway.ChatRequest, requestedModel string) error {
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
			respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
			msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
			seqNum := 0
			reasoningStarted := false
			reasoningStopped := false
			messageStarted := false
			outputIndex := 0

			upstreamStatus, streamErr := h.upstream.Stream(streamCtx, route, body, func(chunk []byte) error {
				data := extractSSEData(chunk)
				if data == "" || data == "[DONE]" {
					return nil
				}

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

					// Reasoning deltas → reasoning output item
					if choice.Delta.Reasoning != "" {
						if !reasoningStarted {
							reasoningStarted = true
							rsID := fmt.Sprintf("rs_%d", time.Now().UnixNano())
							writeResponsesSSE(w, "response.output_item.added", map[string]any{
								"output_index": outputIndex,
								"item": map[string]any{
									"type": "reasoning", "id": rsID,
									"status": "in_progress", "summary": []any{},
								},
								"sequence_number": seqNum,
							})
							seqNum++
							writeResponsesSSE(w, "response.reasoning_summary_part.added", map[string]any{
								"output_index": outputIndex, "summary_index": 0,
								"part":            map[string]string{"type": "summary_text", "text": ""},
								"sequence_number": seqNum,
							})
							seqNum++
						}
						writeResponsesSSE(w, "response.reasoning_summary_text.delta", map[string]any{
							"output_index": outputIndex, "summary_index": 0,
							"delta": choice.Delta.Reasoning, "sequence_number": seqNum,
						})
						seqNum++
					}

					// Text deltas → message output item
					if choice.Delta.Content != nil {
						if text, ok := choice.Delta.Content.(string); ok && text != "" {
							// Close reasoning item if needed
							if reasoningStarted && !reasoningStopped {
								reasoningStopped = true
								writeResponsesSSE(w, "response.reasoning_summary_part.done", map[string]any{
									"output_index": outputIndex, "summary_index": 0,
									"part":            map[string]string{"type": "summary_text", "text": ""},
									"sequence_number": seqNum,
								})
								seqNum++
								writeResponsesSSE(w, "response.output_item.done", map[string]any{
									"output_index": outputIndex,
									"item": map[string]any{
										"type": "reasoning", "status": "completed",
									},
									"sequence_number": seqNum,
								})
								seqNum++
								outputIndex++
							}
							// Start message item if needed
							if !messageStarted {
								messageStarted = true
								writeResponsesSSE(w, "response.output_item.added", map[string]any{
									"output_index": outputIndex,
									"item": map[string]any{
										"type": "message", "id": msgID, "role": "assistant",
										"status": "in_progress", "content": []any{},
									},
									"sequence_number": seqNum,
								})
								seqNum++
								writeResponsesSSE(w, "response.content_part.added", map[string]any{
									"output_index": outputIndex, "content_index": 0,
									"part":            map[string]string{"type": "output_text", "text": ""},
									"sequence_number": seqNum,
								})
								seqNum++
							}
							writeResponsesSSE(w, "response.output_text.delta", map[string]any{
								"output_index": outputIndex, "content_index": 0,
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
				h.fireKeyHealthWebhook(route, upstreamStatus)

				// Mid-stream failure: can't retry
				if headersSent {
					// Ensure message item started for proper close
					if !messageStarted {
						messageStarted = true
						writeResponsesSSE(w, "response.output_item.added", map[string]any{
							"output_index": outputIndex,
							"item": map[string]any{
								"type": "message", "id": msgID, "role": "assistant",
								"status": "in_progress", "content": []any{},
							},
							"sequence_number": seqNum,
						})
						seqNum++
						writeResponsesSSE(w, "response.content_part.added", map[string]any{
							"output_index": outputIndex, "content_index": 0,
							"part":            map[string]string{"type": "output_text", "text": ""},
							"sequence_number": seqNum,
						})
						seqNum++
					}
					writeResponsesSSE(w, "response.content_part.done", map[string]any{
						"output_index": outputIndex, "content_index": 0,
						"part":            map[string]string{"type": "output_text", "text": ""},
						"sequence_number": seqNum,
					})
					seqNum++
					writeResponsesSSE(w, "response.output_item.done", map[string]any{
						"output_index":    outputIndex,
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
					h.logUsage(route, requestedModel, resp, http.StatusBadGateway, duration, true, streamErr)
					if h.metrics != nil {
						h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolResponses, gateway.StatusError, duration)
					}
					return
				}

				// Pre-stream error: can retry
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
				writeResponsesSSE(w, "response.failed", map[string]any{
					"response": map[string]any{
						"id": respID, "object": "response", "created_at": time.Now().Unix(),
						"status": "failed", "model": requestedModel,
					},
					"sequence_number": 0,
				})
				h.logUsage(route, requestedModel, nil, upstreamStatus, duration, true, streamErr)
				if h.metrics != nil {
					h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolResponses, gateway.StatusError, duration)
					h.metrics.ObserveError(route.ProviderID.String(), fmt.Sprintf("%d", upstreamStatus))
				}
				return
			}

			// Success
			h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

			// Close reasoning item if it was started but not closed
			if reasoningStarted && !reasoningStopped {
				reasoningStopped = true
				writeResponsesSSE(w, "response.reasoning_summary_part.done", map[string]any{
					"output_index": outputIndex, "summary_index": 0,
					"part":            map[string]string{"type": "summary_text", "text": ""},
					"sequence_number": seqNum,
				})
				seqNum++
				writeResponsesSSE(w, "response.output_item.done", map[string]any{
					"output_index": outputIndex,
					"item": map[string]any{
						"type": "reasoning", "status": "completed",
					},
					"sequence_number": seqNum,
				})
				seqNum++
				outputIndex++
			}

			// Ensure message item started for proper close
			if !messageStarted {
				messageStarted = true
				writeResponsesSSE(w, "response.output_item.added", map[string]any{
					"output_index": outputIndex,
					"item": map[string]any{
						"type": "message", "id": msgID, "role": "assistant",
						"status": "in_progress", "content": []any{},
					},
					"sequence_number": seqNum,
				})
				seqNum++
				writeResponsesSSE(w, "response.content_part.added", map[string]any{
					"output_index": outputIndex, "content_index": 0,
					"part":            map[string]string{"type": "output_text", "text": ""},
					"sequence_number": seqNum,
				})
				seqNum++
			}

			writeResponsesSSE(w, "response.content_part.done", map[string]any{
				"output_index": outputIndex, "content_index": 0,
				"part":            map[string]string{"type": "output_text", "text": ""},
				"sequence_number": seqNum,
			})
			seqNum++

			status := "completed"
			if lastFinishReason != nil && *lastFinishReason == "length" {
				status = "incomplete"
			}
			writeResponsesSSE(w, "response.output_item.done", map[string]any{
				"output_index":    outputIndex,
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

			if h.metrics != nil {
				h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), gateway.ProtocolResponses, gateway.StatusOK, duration)
				if resp != nil && resp.Usage != nil {
					h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				}
			}

			h.logUsage(route, requestedModel, resp, http.StatusOK, duration, true, nil)
			return
		}

		// All attempts exhausted
		writeResponsesSSE(w, "response.failed", map[string]any{
			"response": map[string]any{
				"id": fmt.Sprintf("resp_%d", time.Now().UnixNano()), "object": "response",
				"created_at": time.Now().Unix(), "status": "failed", "model": requestedModel,
			},
			"sequence_number": 0,
		})
		if len(routes) > 0 {
			h.logUsage(routes[0], requestedModel, nil, lastStatus, 0, true, lastErr)
		}
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
						combined := texts[0]
						for i := 1; i < len(texts); i++ {
							combined += "\n" + texts[i]
						}
						msg.Content = combined
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
			// Reasoning output item (emitted before message)
			if choice.Message.Reasoning != "" {
				reasoningOutput := respOutput{
					Type:   "reasoning",
					ID:     fmt.Sprintf("rs_%d", time.Now().UnixNano()),
					Status: "completed",
					Summary: []respReasoningSummary{{
						Type: "summary_text",
						Text: choice.Message.Reasoning,
					}},
				}
				rr.Output = append(rr.Output, reasoningOutput)
			}

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
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	_ = w.Flush()
}

// responsesError returns an error in the OpenAI error envelope format.
func responsesError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error": fiber.Map{
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}

// responsesErrorFromErrx converts an internal error (typically *errx.Error)
// into a Responses-API-shaped error response.
func responsesErrorFromErrx(c *fiber.Ctx, err error) error {
	status, message := errxStatusAndMessage(err)
	return responsesError(c, status, message)
}

// extractResponsesTexts extracts text content from Responses API input.
func extractResponsesTexts(req *responsesRequest) []string {
	var texts []string
	if req.Instructions != "" {
		texts = append(texts, req.Instructions)
	}
	if v, ok := req.Input.(string); ok {
		texts = append(texts, v)
	}
	return texts
}
