package gatewayapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/gofiber/fiber/v2"
)

// Moderations handles POST /v1/moderations (OpenAI-compatible).
// Proxied to the upstream provider; free upstream models cost nothing unless
// the mapping defines a request price.
func (h *GatewayHandlers) Moderations(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	tenantID := authCtx.TenantID

	var req gateway.ModerationRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Input == nil {
		return fiber.NewError(fiber.StatusBadRequest, "input is required")
	}
	requestedModel := req.Model
	if requestedModel == "" {
		requestedModel = "omni-moderation-latest"
	}

	if err := h.checkModelAccess(authCtx, requestedModel); err != nil {
		return err
	}
	if err := h.checkRateLimit(c, tenantID); err != nil {
		return err
	}
	defer h.rateLimiter.Release(c.Context(), tenantID.String())

	route, err := h.router.Resolve(c.Context(), requestedModel, &tenantID, nil)
	if err != nil {
		return err
	}

	req.Model = route.ExternalID
	body, _ := json.Marshal(req)

	start := time.Now()
	modResp, statusCode, err := h.upstream.CallModeration(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.healthTracker.ReportError(route.KeyID, statusCode)
		h.logModalityRequest(tenantID, route, requestedModel, "moderation", nil, statusCode, duration, err, nil)
		return fiber.NewError(fiber.StatusBadGateway, "moderation request failed")
	}
	h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "moderation", "ok", duration)
	}

	content := &usage.RequestContent{IsDebug: isDebugMode(c)}
	if content.IsDebug {
		content.RawRequest = json.RawMessage(c.Body())
		raw, _ := json.Marshal(modResp)
		content.RawResponse = raw
	}
	h.logModalityRequest(tenantID, route, requestedModel, "moderation", nil, http.StatusOK, duration, nil, content)

	modResp.Model = requestedModel
	return c.Status(http.StatusOK).JSON(modResp)
}

// Rerank handles POST /v1/rerank (Cohere v2-compatible).
// Billed per search unit (1 unit per 100 documents).
func (h *GatewayHandlers) Rerank(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	tenantID := authCtx.TenantID

	var req gateway.RerankRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Model == "" {
		return fiber.NewError(fiber.StatusBadRequest, "model is required")
	}
	if req.Query == "" {
		return fiber.NewError(fiber.StatusBadRequest, "query is required")
	}
	if len(req.Documents) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "documents are required")
	}

	requestedModel := req.Model

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

	route, err := h.router.Resolve(c.Context(), requestedModel, &tenantID, nil)
	if err != nil {
		return err
	}

	req.Model = route.ExternalID
	body, _ := json.Marshal(req)

	start := time.Now()
	rrResp, statusCode, err := h.upstream.CallRerank(c.Context(), route, body)
	duration := time.Since(start)

	if err != nil {
		h.healthTracker.ReportError(route.KeyID, statusCode)
		h.logModalityRequest(tenantID, route, requestedModel, "rerank", nil, statusCode, duration, err, nil)
		return fiber.NewError(fiber.StatusBadGateway, "rerank request failed")
	}
	h.healthTracker.ReportSuccessWithLatency(route.KeyID, duration)

	cost := gateway.CalculateRerankCost(route, len(req.Documents))
	h.debitUsage(c.Context(), tenantID, authCtx.WalletID, cost)

	if h.metrics != nil {
		h.metrics.ObserveRequest(requestedModel, route.ProviderID.String(), "rerank", "ok", duration)
	}

	respMeta, _ := json.Marshal(map[string]any{
		"documents": len(req.Documents),
		"results":   len(rrResp.Results),
	})
	content := &usage.RequestContent{ResponseBody: respMeta, IsDebug: isDebugMode(c)}
	if content.IsDebug {
		content.RawRequest = json.RawMessage(c.Body())
		raw, _ := json.Marshal(rrResp)
		content.RawResponse = raw
	}
	h.logModalityRequest(tenantID, route, requestedModel, "rerank", nil, http.StatusOK, duration, nil, content)

	h.fireModalityWebhook(tenantID, requestedModel, route, "rerank", cost, duration)

	return c.Status(http.StatusOK).JSON(rrResp)
}
