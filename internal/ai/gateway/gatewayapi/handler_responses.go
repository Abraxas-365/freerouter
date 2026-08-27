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
	Model           string    `json:"model"`
	Input           any       `json:"input"`            // string or []inputItem
	Instructions    string    `json:"instructions,omitempty"`
	MaxOutputTokens *int      `json:"max_output_tokens,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	TopP            *float64  `json:"top_p,omitempty"`
	Stream          bool      `json:"stream,omitempty"`
	Tools           []any     `json:"tools,omitempty"`
	ToolChoice      any       `json:"tool_choice,omitempty"`
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
	ID        string         `json:"id"`
	Object    string         `json:"object"` // "response"
	CreatedAt int64          `json:"created_at"`
	Status    string         `json:"status"` // "completed", "incomplete", "failed"
	Output    []respOutput   `json:"output"`
	Model     string         `json:"model"`
	Usage     *respUsage     `json:"usage,omitempty"`
}

type respOutput struct {
	Type    string           `json:"type"` // "message", "function_call"
	ID      string           `json:"id,omitempty"`
	Role    string           `json:"role,omitempty"` // for message
	Status  string           `json:"status"`
	Content []respContent    `json:"content,omitempty"` // for message
	CallID  string           `json:"call_id,omitempty"` // for function_call
	Name    string           `json:"name,omitempty"`    // for function_call
	Args    string           `json:"arguments,omitempty"` // for function_call
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

	var req responsesRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	requestedModel := req.Model

	// Convert to internal ChatRequest
	chatReq := responsesToChatRequest(&req)

	// Resolve route
	tenantID := &authCtx.TenantID
	route, err := h.router.Resolve(c.Context(), chatReq.Model, tenantID, chatReq.MaxTokens)
	if err != nil {
		return err
	}

	chatReq.Model = route.ExternalID
	body, _ := json.Marshal(chatReq)

	if req.Stream {
		return h.handleResponsesStream(c, route, requestedModel, authCtx.TenantID, body)
	}
	return h.handleResponsesNonStream(c, route, requestedModel, authCtx.TenantID, body)
}

func (h *GatewayHandlers) handleResponsesNonStream(c *fiber.Ctx, route *gateway.RouteResult, requestedModel string, tenantID kernel.TenantID, body []byte) error {
	start := time.Now()

	resp, statusCode, err := h.upstream.Call(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.usage.LogRequest(tenantID, route, requestedModel, nil, statusCode, duration, false, err)
		return err
	}

	cost := calculateCost(route, resp)
	if cost > 0 {
		if _, err := h.billing.DebitUsage(c.Context(), tenantID, cost, ""); err != nil {
			slog.Error("failed to debit usage", "tenant_id", tenantID, "cost", cost, "error", err)
		}
	}

	h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, false, nil)

	// Convert OpenAI response → Responses API format
	respAPI := chatResponseToResponses(resp, requestedModel)
	return c.JSON(respAPI)
}

func (h *GatewayHandlers) handleResponsesStream(c *fiber.Ctx, route *gateway.RouteResult, requestedModel string, tenantID kernel.TenantID, body []byte) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		start := time.Now()
		var accumulatedUsage *gateway.Usage
		var lastFinishReason *string
		respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
		msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
		var seqNum int

		// response.created
		writeResponsesSSE(w, "response.created", map[string]any{
			"response": map[string]any{
				"id":         respID,
				"object":     "response",
				"created_at": time.Now().Unix(),
				"status":     "in_progress",
				"output":     []any{},
				"model":      requestedModel,
			},
			"sequence_number": seqNum,
		})
		seqNum++

		// output_item.added (message)
		writeResponsesSSE(w, "response.output_item.added", map[string]any{
			"output_index": 0,
			"item": map[string]any{
				"type":    "message",
				"id":      msgID,
				"role":    "assistant",
				"status":  "in_progress",
				"content": []any{},
			},
			"sequence_number": seqNum,
		})
		seqNum++

		// content_part.added
		writeResponsesSSE(w, "response.content_part.added", map[string]any{
			"output_index":  0,
			"content_index": 0,
			"part":          map[string]string{"type": "output_text", "text": ""},
			"sequence_number": seqNum,
		})
		seqNum++

		streamErr := h.upstream.Stream(c.Context(), route, body, func(chunk []byte) error {
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
					text, ok := choice.Delta.Content.(string)
					if ok && text != "" {
						writeResponsesSSE(w, "response.output_text.delta", map[string]any{
							"output_index":    0,
							"content_index":   0,
							"delta":           text,
							"sequence_number": seqNum,
						})
						seqNum++
					}
				}
			}
			return nil
		})

		duration := time.Since(start)

		// content_part.done
		writeResponsesSSE(w, "response.content_part.done", map[string]any{
			"output_index":    0,
			"content_index":   0,
			"part":            map[string]string{"type": "output_text", "text": ""},
			"sequence_number": seqNum,
		})
		seqNum++

		// output_item.done
		status := "completed"
		if lastFinishReason != nil && *lastFinishReason == "length" {
			status = "incomplete"
		}
		writeResponsesSSE(w, "response.output_item.done", map[string]any{
			"output_index": 0,
			"item": map[string]any{
				"type":   "message",
				"id":     msgID,
				"role":   "assistant",
				"status": status,
			},
			"sequence_number": seqNum,
		})
		seqNum++

		// response.completed
		completedResp := map[string]any{
			"id":         respID,
			"object":     "response",
			"created_at": time.Now().Unix(),
			"status":     status,
			"model":      requestedModel,
		}
		if accumulatedUsage != nil {
			completedResp["usage"] = map[string]int{
				"input_tokens":  accumulatedUsage.PromptTokens,
				"output_tokens": accumulatedUsage.CompletionTokens,
				"total_tokens":  accumulatedUsage.TotalTokens,
			}
		}

		eventType := "response.completed"
		if streamErr != nil {
			eventType = "response.failed"
			completedResp["status"] = "failed"
		}
		writeResponsesSSE(w, eventType, map[string]any{
			"response":        completedResp,
			"sequence_number": seqNum,
		})

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
