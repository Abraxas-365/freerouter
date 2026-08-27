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
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type GatewayHandlers struct {
	router      *gateway.Router
	upstream    *gateway.Upstream
	usage       *usagesrv.UsageService
	billing     *billingsrv.BillingService
	modelRepo   provider.ModelRepository
	mappingRepo provider.MappingRepository
}

func NewGatewayHandlers(router *gateway.Router, upstream *gateway.Upstream, usage *usagesrv.UsageService, billing *billingsrv.BillingService, modelRepo provider.ModelRepository, mappingRepo provider.MappingRepository) *GatewayHandlers {
	return &GatewayHandlers{
		router:      router,
		upstream:    upstream,
		usage:       usage,
		billing:     billing,
		modelRepo:   modelRepo,
		mappingRepo: mappingRepo,
	}
}

func (h *GatewayHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	v1 := router.Group("/v1", authMiddleware.Authenticate())
	v1.Get("/models", authMiddleware.RequireScope("gateway:read"), h.ListModels)
	v1.Post("/chat/completions", authMiddleware.RequireScope("gateway:chat"), h.ChatCompletions)
}

// ListModels returns all active models in OpenAI-compatible format.
func (h *GatewayHandlers) ListModels(c *fiber.Ctx) error {
	models, err := h.modelRepo.FindActive(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list models")
	}

	type openAIModel struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	data := make([]openAIModel, 0, len(models))
	for _, m := range models {
		data = append(data, openAIModel{
			ID:      m.ID.String(),
			Object:  "model",
			Created: m.CreatedAt.Unix(),
			OwnedBy: m.Family,
		})
	}

	return c.JSON(fiber.Map{
		"object": "list",
		"data":   data,
	})
}

func (h *GatewayHandlers) ChatCompletions(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	var req gateway.ChatRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	requestedModel := req.Model

	// Route: resolve model -> provider -> credential
	tenantID := &authCtx.TenantID
	route, err := h.router.Resolve(c.Context(), req.Model, tenantID, req.MaxTokens)
	if err != nil {
		return err
	}

	// Rewrite model to provider's external ID
	req.Model = route.ExternalID

	body, err := json.Marshal(req)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to marshal request")
	}

	if req.Stream {
		return h.handleStream(c, route, requestedModel, authCtx.TenantID, body)
	}
	return h.handleNonStream(c, route, requestedModel, authCtx.TenantID, body)
}

func (h *GatewayHandlers) handleNonStream(c *fiber.Ctx, route *gateway.RouteResult, requestedModel string, tenantID kernel.TenantID, body []byte) error {
	start := time.Now()

	resp, statusCode, err := h.upstream.Call(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.usage.LogRequest(tenantID, route, requestedModel, nil, statusCode, duration, false, err)
		return err
	}

	// Debit tenant balance for token usage
	cost := calculateCost(route, resp)
	if cost > 0 {
		if _, err := h.billing.DebitUsage(c.Context(), tenantID, cost, ""); err != nil {
			slog.Error("failed to debit usage", "tenant_id", tenantID, "cost", cost, "error", err)
		}
	}

	h.usage.LogRequest(tenantID, route, requestedModel, resp, http.StatusOK, duration, false, nil)
	return c.JSON(resp)
}

func (h *GatewayHandlers) handleStream(c *fiber.Ctx, route *gateway.RouteResult, requestedModel string, tenantID kernel.TenantID, body []byte) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		start := time.Now()
		var lastChunk gateway.ChatStreamChunk

		streamErr := h.upstream.Stream(c.Context(), route, body, func(chunk []byte) error {
			// Capture last chunk for usage data (final chunk has usage stats)
			_ = json.Unmarshal(chunk, &lastChunk)

			if _, err := fmt.Fprintf(w, "data: %s\n\n", chunk); err != nil {
				return err
			}
			return w.Flush()
		})

		duration := time.Since(start)

		// Build a ChatResponse from the last chunk's usage for logging
		var resp *gateway.ChatResponse
		if lastChunk.Usage != nil {
			resp = &gateway.ChatResponse{
				Usage:   lastChunk.Usage,
				Choices: lastChunk.Choices,
			}
		}

		statusCode := http.StatusOK
		if streamErr != nil {
			statusCode = http.StatusBadGateway
			errJSON, _ := json.Marshal(fiber.Map{"error": streamErr.Error()})
			fmt.Fprintf(w, "data: %s\n\n", errJSON)
			w.Flush()
		} else {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			w.Flush()
		}

		// Debit tenant balance for token usage
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

// calculateCost computes the total cost in USD for a request based on token
// counts and the mapping's per-million-token pricing.
func calculateCost(route *gateway.RouteResult, resp *gateway.ChatResponse) float64 {
	if resp == nil || resp.Usage == nil {
		return 0
	}
	var cost float64
	if route.InputPrice != nil {
		cost += float64(resp.Usage.PromptTokens) * *route.InputPrice / 1_000_000
	}
	if route.OutputPrice != nil {
		cost += float64(resp.Usage.CompletionTokens) * *route.OutputPrice / 1_000_000
	}
	return cost
}
