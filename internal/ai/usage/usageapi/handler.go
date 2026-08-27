package usageapi

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
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
	u.Get("/logs", authMiddleware.RequireScope("usage:read"), h.QueryLogs)
	u.Get("/logs/:id", authMiddleware.RequireScope("usage:read"), h.GetLog)
	u.Get("/summary", authMiddleware.RequireScope("usage:read"), h.GetSummary)
}

func (h *UsageHandlers) GetLog(c *fiber.Ctx) error {
	log, err := h.service.GetLog(c.Context(), kernel.NewUsageLogID(c.Params("id")))
	if err != nil {
		return err
	}
	return c.JSON(log.ToDTO())
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
