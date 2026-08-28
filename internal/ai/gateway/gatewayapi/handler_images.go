package gatewayapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

// ImageGeneration handles POST /v1/images/generations.
// It proxies the request to the upstream provider and bills per image.
func (h *GatewayHandlers) ImageGeneration(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	tenantID := authCtx.TenantID

	// Parse request
	var req gateway.ImageRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Prompt == "" {
		return fiber.NewError(fiber.StatusBadRequest, "prompt is required")
	}
	if req.Model == "" {
		req.Model = "dall-e-2" // OpenAI default
	}

	requestedModel := req.Model

	// Per-key model restriction
	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}

	// Record request metric
	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, "", "images", "started", 0)
	}

	// Pre-flight checks
	if err := h.checkSpendingLimit(c, tenantID); err != nil {
		return err
	}
	if err := h.checkRateLimit(c, tenantID); err != nil {
		return err
	}

	// Resolve route
	route, err := h.router.Resolve(c.Context(), requestedModel, &tenantID, nil)
	if err != nil {
		if h.metrics != nil {
			h.metrics.ObserveError(route.ProviderID.String(), "route_error")
		}
		return err
	}

	// Rewrite model to external ID
	req.Model = route.ExternalID
	body, _ := json.Marshal(req)

	// Call upstream
	start := time.Now()
	imageResp, statusCode, err := h.upstream.CallImage(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.healthTracker.ReportError(route.KeyID, statusCode)
		if h.metrics != nil {
			h.metrics.ObserveError(route.ProviderID.String(), "upstream_error")
		}
		h.logImageRequest(tenantID, route, requestedModel, nil, statusCode, duration, err, nil)
		return fiber.NewError(fiber.StatusBadGateway, "image generation failed")
	}

	h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

	// Calculate cost (per image, not per token)
	numImages := 1
	if req.N != nil && *req.N > 0 {
		numImages = *req.N
	}
	perImage := gateway.DefaultImagePricing(route.ExternalID, req.Size, req.Quality)
	totalCost := perImage * float64(numImages)

	// Debit billing
	if totalCost > 0 {
		if _, err := h.billing.DebitUsage(c.Context(), tenantID, totalCost, ""); err != nil {
			slog.Error("image: failed to debit usage", "error", err, "tenant_id", tenantID, "cost", totalCost)
		}
	}

	// Record metrics
	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "images", "success", duration)
		h.metrics.ObserveTokens(requestedModel, route.ProviderID.String(), 0, 0)
	}

	// Restore requested model in response
	respJSON, _ := json.Marshal(imageResp)

	// Log usage
	content := h.buildImageContent(c, &req, imageResp)
	h.logImageRequest(tenantID, route, requestedModel, imageResp, http.StatusOK, duration, nil, content)

	// Fire webhook
	h.fireImageWebhook(tenantID, requestedModel, route, numImages, totalCost, duration)

	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).Send(respJSON)
}

// logImageRequest logs an image generation request to the usage system.
func (h *GatewayHandlers) logImageRequest(
	tenantID kernel.TenantID,
	route *gateway.RouteResult,
	requestedModel string,
	resp *gateway.ImageResponse,
	statusCode int,
	duration time.Duration,
	reqErr error,
	content *usage.RequestContent,
) {
	// Build a synthetic ChatResponse for the usage logger
	var chatResp *gateway.ChatResponse
	if resp != nil {
		numImages := len(resp.Data)
		chatResp = &gateway.ChatResponse{
			Object: "image.generation",
			Usage: &gateway.Usage{
				TotalTokens: numImages, // abuse total_tokens to store image count
			},
		}
		if resp.Usage != nil {
			chatResp.Usage.PromptTokens = resp.Usage.InputTokens
			chatResp.Usage.CompletionTokens = resp.Usage.OutputTokens
			chatResp.Usage.TotalTokens = resp.Usage.TotalTokens
		}
	}
	h.usage.LogRequest(tenantID, route, requestedModel, chatResp, statusCode, duration, false, reqErr, content)
}

// buildImageContent builds request content for logging.
func (h *GatewayHandlers) buildImageContent(c *fiber.Ctx, req *gateway.ImageRequest, resp *gateway.ImageResponse) *usage.RequestContent {
	reqJSON, _ := json.Marshal(req)
	respJSON, _ := json.Marshal(resp)
	content := &usage.RequestContent{
		Messages:     reqJSON,
		ResponseBody: respJSON,
		IsDebug:      isDebugMode(c),
	}
	if content.IsDebug {
		content.RawRequest = json.RawMessage(c.Body())
		content.RawResponse = respJSON
	}
	return content
}

// fireImageWebhook fires a webhook event for a completed image generation.
func (h *GatewayHandlers) fireImageWebhook(
	tenantID kernel.TenantID,
	requestedModel string,
	route *gateway.RouteResult,
	numImages int,
	totalCost float64,
	duration time.Duration,
) {
	if h.webhooks == nil {
		return
	}
	h.webhooks.Fire(tenantID, webhook.EventRequestCompleted, map[string]any{
		"type":            "image_generation",
		"requested_model": requestedModel,
		"provider":        route.ProviderID.String(),
		"num_images":      numImages,
		"total_cost_usd":  totalCost,
		"duration_ms":     duration.Milliseconds(),
	})
}
