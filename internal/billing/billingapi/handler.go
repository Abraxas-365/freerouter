package billingapi

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type BillingHandlers struct {
	service *billingsrv.BillingService
}

func NewBillingHandlers(service *billingsrv.BillingService) *BillingHandlers {
	return &BillingHandlers{service: service}
}

func (h *BillingHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	b := router.Group("/billing", authMiddleware.Authenticate())

	b.Get("/balance", authMiddleware.RequireScope("billing:read"), h.GetBalance)
	b.Post("/top-up", authMiddleware.RequireScope("billing:write"), h.TopUp)
	b.Post("/adjust", authMiddleware.RequireScope("billing:admin"), h.Adjust)
	b.Get("/transactions", authMiddleware.RequireScope("billing:read"), h.ListTransactions)

	// Spending limits
	sl := router.Group("/spending-limits", authMiddleware.Authenticate())
	sl.Get("/:tenantId", authMiddleware.RequireScope("billing:read"), h.GetSpendingLimit)
	sl.Put("/:tenantId", authMiddleware.RequireScope("billing:write"), h.UpsertSpendingLimit)
	sl.Delete("/:tenantId", authMiddleware.RequireScope("billing:write"), h.DeleteSpendingLimit)
	sl.Get("/:tenantId/check", authMiddleware.RequireScope("billing:read"), h.CheckSpendingLimit)
}

func (h *BillingHandlers) GetBalance(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	balance, err := h.service.GetBalance(c.Context(), authCtx.TenantID)
	if err != nil {
		return err
	}
	return c.JSON(balance.ToDTO())
}

func (h *BillingHandlers) TopUp(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[billing.TopUpRequest](c)
	if err != nil {
		return err
	}

	balance, tx, err := h.service.TopUp(c.Context(), authCtx.TenantID, req)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"balance":     balance.ToDTO(),
		"transaction": tx.ToDTO(),
	})
}

func (h *BillingHandlers) Adjust(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	req, err := kernel.BindAndValidate[billing.AdjustRequest](c)
	if err != nil {
		return err
	}

	balance, tx, err := h.service.Adjust(c.Context(), authCtx.TenantID, req)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"balance":     balance.ToDTO(),
		"transaction": tx.ToDTO(),
	})
}

func (h *BillingHandlers) ListTransactions(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	q := billing.TransactionQuery{
		TenantID: authCtx.TenantID,
		Type:     billing.TransactionType(c.Query("type")),
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

	response, err := h.service.QueryTransactions(c.Context(), q)
	if err != nil {
		return err
	}
	return c.JSON(response)
}

// ============================================================================
// Spending Limit handlers
// ============================================================================

func (h *BillingHandlers) GetSpendingLimit(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	cfg, err := h.service.GetSpendingLimit(c.Context(), tenantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to get spending limit")
	}
	if cfg == nil {
		return c.JSON(fiber.Map{
			"tenant_id":         tenantID,
			"daily_limit_usd":   nil,
			"monthly_limit_usd": nil,
			"message":           "no spending limits configured",
		})
	}
	return c.JSON(cfg)
}

func (h *BillingHandlers) UpsertSpendingLimit(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	var req billing.UpsertSpendingLimitRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	cfg, err := h.service.UpsertSpendingLimit(c.Context(), tenantID, req)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to save spending limit")
	}
	return c.JSON(cfg)
}

func (h *BillingHandlers) DeleteSpendingLimit(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	if err := h.service.DeleteSpendingLimit(c.Context(), tenantID); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete spending limit")
	}
	return c.JSON(fiber.Map{"message": "Spending limit deleted"})
}

func (h *BillingHandlers) CheckSpendingLimit(c *fiber.Ctx) error {
	tenantID := kernel.NewTenantID(c.Params("tenantId"))

	result, err := h.service.CheckSpendingLimit(c.Context(), tenantID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to check spending limit")
	}
	return c.JSON(result)
}
