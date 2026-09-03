package usageapi

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/iam/scopes"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type UsageHandlers struct {
	service *usagesrv.UsageService
}

func NewUsageHandlers(service *usagesrv.UsageService) *UsageHandlers {
	return &UsageHandlers{service: service}
}

func (h *UsageHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	u := router.Group("/usage", authMiddleware.Authenticate())
	u.Get("/logs", authMiddleware.RequireScope(scopes.ScopeUsageRead), h.QueryLogs)
	u.Get("/logs/:id", authMiddleware.RequireScope(scopes.ScopeUsageRead), h.GetLog)
	u.Get("/summary", authMiddleware.RequireScope(scopes.ScopeUsageRead), h.GetSummary)

	u.Get("/retention/:tenantId", authMiddleware.RequireScope(scopes.ScopeUsageRead), auth.ValidateTenantAccess(), h.GetRetentionConfig)
	u.Put("/retention/:tenantId", authMiddleware.RequireScope(scopes.ScopeUsageWrite), auth.ValidateTenantAccess(), h.UpsertRetentionConfig)
	u.Delete("/retention/:tenantId", authMiddleware.RequireScope(scopes.ScopeUsageWrite), auth.ValidateTenantAccess(), h.DeleteRetentionConfig)
}

func (h *UsageHandlers) GetLog(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	log, err := h.service.GetLog(c.Context(), kernel.NewUsageLogID(c.Params("id")))
	if err != nil {
		return err
	}
	if log.TenantID != authCtx.TenantID && !authCtx.HasScope("*") {
		return fiber.NewError(fiber.StatusForbidden, "access denied to this log")
	}
	return c.JSON(log.ToDetailDTO())
}

func (h *UsageHandlers) QueryLogs(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	q := usage.UsageQuery{
		TenantID: authCtx.TenantID,
		Model:    c.Query("model"),
		Provider: c.Query("provider"),
		Limit:    c.QueryInt("limit", 100),
		Offset:   c.QueryInt("offset", 0),
	}

	if from := c.Query("from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid 'from' date format, use RFC3339")
		}
		q.From = &t
	}
	if to := c.Query("to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid 'to' date format, use RFC3339")
		}
		q.To = &t
	}

	response, err := h.service.QueryLogs(c.Context(), q)
	if err != nil {
		return err
	}
	return c.JSON(response)
}

func (h *UsageHandlers) GetSummary(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	var from, to *time.Time
	if f := c.Query("from"); f != "" {
		t, err := time.Parse(time.RFC3339, f)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid 'from' date format, use RFC3339")
		}
		from = &t
	}
	if t := c.Query("to"); t != "" {
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid 'to' date format, use RFC3339")
		}
		to = &parsed
	}

	response, err := h.service.GetUsageSummary(c.Context(), authCtx.TenantID, from, to)
	if err != nil {
		return err
	}
	return c.JSON(response)
}

// ============================================================================
// Data Retention Config
// ============================================================================

type upsertRetentionConfigRequest struct {
	RetentionDays       int  `json:"retention_days"`
	RetainMessages      bool `json:"retain_messages"`
	RetainResponseBody  bool `json:"retain_response_body"`
	RetainDebugPayloads bool `json:"retain_debug_payloads"`
}

func (h *UsageHandlers) GetRetentionConfig(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	cfg, err := h.service.GetRetentionConfig(c.Context(), tenantID)
	if err != nil {
		return err
	}
	if cfg == nil {
		return c.JSON(usage.DataRetentionConfig{
			TenantID:            tenantID,
			RetentionDays:       30,
			RetainMessages:      true,
			RetainResponseBody:  true,
			RetainDebugPayloads: false,
		})
	}
	return c.JSON(cfg)
}

func (h *UsageHandlers) UpsertRetentionConfig(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	var req upsertRetentionConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	cfg := usage.DataRetentionConfig{
		TenantID:            tenantID,
		RetentionDays:       req.RetentionDays,
		RetainMessages:      req.RetainMessages,
		RetainResponseBody:  req.RetainResponseBody,
		RetainDebugPayloads: req.RetainDebugPayloads,
	}

	saved, err := h.service.UpsertRetentionConfig(c.Context(), cfg)
	if err != nil {
		return err
	}
	return c.JSON(saved)
}

func (h *UsageHandlers) DeleteRetentionConfig(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	if err := h.service.DeleteRetentionConfig(c.Context(), tenantID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "Data retention config deleted, tenant will use defaults"})
}
