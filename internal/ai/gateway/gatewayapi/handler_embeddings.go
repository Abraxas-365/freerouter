package gatewayapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

// Embeddings handles POST /v1/embeddings.
// It proxies the request to the upstream provider and bills per token.
func (h *GatewayHandlers) Embeddings(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	tenantID := authCtx.TenantID

	// Parse request
	var req gateway.EmbeddingRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Input == nil {
		return fiber.NewError(fiber.StatusBadRequest, "input is required")
	}
	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}

	requestedModel := req.Model

	// Pre-flight checks
	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}
	if err := h.checkSpendingLimit(c, tenantID); err != nil {
		return err
	}
	if err := h.checkWalletBalance(c, authCtx); err != nil {
		return err
	}
	if err := h.checkRateLimit(c, tenantID); err != nil {
		return err
	}
	defer h.rateLimiter.Release(c.Context(), tenantID.String())

	// Resolve route
	route, err := h.router.Resolve(c.Context(), requestedModel, &tenantID, nil)
	if err != nil {
		return err
	}

	// Rewrite model to external ID
	req.Model = route.ExternalID
	body, _ := json.Marshal(req)

	// Call upstream
	start := time.Now()
	embResp, statusCode, err := h.upstream.CallEmbedding(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.healthTracker.ReportError(route.KeyID, statusCode)
		h.logEmbeddingRequest(tenantID, route, requestedModel, nil, statusCode, duration, err, nil)
		return fiber.NewError(fiber.StatusBadGateway, "embedding request failed")
	}

	h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

	// Calculate cost using the mapping's token pricing
	cost := calculateEmbeddingCost(route, &embResp.Usage)
	h.debitUsage(c.Context(), tenantID, authCtx.WalletID, cost)

	// Metrics
	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "embeddings", "ok", duration)
		h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), embResp.Usage.PromptTokens, 0)
	}

	// Restore requested model in response
	embResp.Model = requestedModel
	respJSON, _ := json.Marshal(embResp)

	// Log usage
	content := h.buildEmbeddingContent(c, &req, embResp)
	h.logEmbeddingRequest(tenantID, route, requestedModel, embResp, http.StatusOK, duration, nil, content)

	// Fire webhook
	h.fireEmbeddingWebhook(tenantID, requestedModel, route, &embResp.Usage, cost, duration)

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(respJSON)
}

func calculateEmbeddingCost(route *gateway.RouteResult, u *gateway.EmbeddingUsage) float64 {
	if u == nil || route.InputPrice == nil {
		return 0
	}
	return float64(u.PromptTokens) * *route.InputPrice / 1_000_000
}

func (h *GatewayHandlers) logEmbeddingRequest(
	tenantID kernel.TenantID,
	route *gateway.RouteResult,
	requestedModel string,
	resp *gateway.EmbeddingResponse,
	statusCode int,
	duration time.Duration,
	reqErr error,
	content *usage.RequestContent,
) {
	var chatResp *gateway.ChatResponse
	if resp != nil {
		chatResp = &gateway.ChatResponse{
			Object: "embedding",
			Usage: &gateway.Usage{
				PromptTokens: resp.Usage.PromptTokens,
				TotalTokens:  resp.Usage.TotalTokens,
			},
		}
	}
	h.usage.LogRequest(tenantID, route, requestedModel, chatResp, statusCode, duration, false, reqErr, content)
}

func (h *GatewayHandlers) buildEmbeddingContent(c *fiber.Ctx, req *gateway.EmbeddingRequest, resp *gateway.EmbeddingResponse) *usage.RequestContent {
	reqJSON, _ := json.Marshal(req)
	// Don't store embedding vectors in response_body — they're huge
	respMeta, _ := json.Marshal(map[string]any{
		"object": resp.Object,
		"model":  resp.Model,
		"usage":  resp.Usage,
		"count":  len(resp.Data),
	})
	content := &usage.RequestContent{
		Messages:     reqJSON,
		ResponseBody: respMeta,
		IsDebug:      isDebugMode(c),
	}
	if content.IsDebug {
		content.RawRequest = json.RawMessage(c.Body())
		fullResp, _ := json.Marshal(resp)
		content.RawResponse = fullResp
	}
	return content
}

func (h *GatewayHandlers) fireEmbeddingWebhook(
	tenantID kernel.TenantID,
	requestedModel string,
	route *gateway.RouteResult,
	u *gateway.EmbeddingUsage,
	cost float64,
	duration time.Duration,
) {
	if h.webhooks == nil {
		return
	}
	h.webhooks.Fire(tenantID, webhook.EventRequestCompleted, map[string]any{
		"type":            "embedding",
		"requested_model": requestedModel,
		"provider":        route.ProviderID.String(),
		"prompt_tokens":   u.PromptTokens,
		"total_tokens":    u.TotalTokens,
		"total_cost_usd":  cost,
		"duration_ms":     duration.Milliseconds(),
	})
}
