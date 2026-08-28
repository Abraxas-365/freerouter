package guardrailsapi

import (
	"encoding/json"
	"strconv"

	"github.com/Abraxas-365/freerouter/internal/ai/guardrails"
	"github.com/Abraxas-365/freerouter/internal/ai/guardrails/guardrailssrv"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/iam/scopes"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type GuardrailHandlers struct {
	service *guardrailssrv.GuardrailsService
}

func NewGuardrailHandlers(service *guardrailssrv.GuardrailsService) *GuardrailHandlers {
	return &GuardrailHandlers{service: service}
}

func (h *GuardrailHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	g := router.Group("/guardrails", authMiddleware.Authenticate())

	// Config
	g.Get("/config", authMiddleware.RequireScope(scopes.ScopeGuardrailsRead), h.GetConfig)
	g.Put("/config", authMiddleware.RequireScope(scopes.ScopeGuardrailsWrite), h.UpsertConfig)

	// Rules
	g.Get("/rules", authMiddleware.RequireScope(scopes.ScopeGuardrailsRead), h.ListRules)
	g.Post("/rules", authMiddleware.RequireScope(scopes.ScopeGuardrailsWrite), h.CreateRule)
	g.Put("/rules/:ruleId", authMiddleware.RequireScope(scopes.ScopeGuardrailsWrite), h.UpdateRule)
	g.Delete("/rules/:ruleId", authMiddleware.RequireScope(scopes.ScopeGuardrailsWrite), h.DeleteRule)

	// Violations
	g.Get("/violations", authMiddleware.RequireScope(scopes.ScopeGuardrailsRead), h.ListViolations)

	// Test
	g.Post("/test", authMiddleware.RequireScope(scopes.ScopeGuardrailsRead), h.TestCheck)
}

func (h *GuardrailHandlers) GetConfig(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	config, err := h.service.GetConfig(c.Context(), authCtx.TenantID)
	if err != nil {
		return err
	}
	return c.JSON(config)
}

func (h *GuardrailHandlers) UpsertConfig(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	var req guardrails.UpsertConfigRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	config, err := h.service.UpsertConfig(c.Context(), authCtx.TenantID, req)
	if err != nil {
		return err
	}
	return c.JSON(config)
}

func (h *GuardrailHandlers) ListRules(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	rules, err := h.service.ListRules(c.Context(), authCtx.TenantID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"rules": rules, "total": len(rules)})
}

func (h *GuardrailHandlers) CreateRule(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	var req guardrails.CreateRuleRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return err
	}

	rule, err := h.service.CreateRule(c.Context(), authCtx.TenantID, req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(rule)
}

func (h *GuardrailHandlers) UpdateRule(c *fiber.Ctx) error {
	ruleID := kernel.NewGuardrailRuleID(c.Params("ruleId"))

	var req guardrails.UpdateRuleRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	rule, err := h.service.UpdateRule(c.Context(), ruleID, req)
	if err != nil {
		return err
	}
	return c.JSON(rule)
}

func (h *GuardrailHandlers) DeleteRule(c *fiber.Ctx) error {
	ruleID := kernel.NewGuardrailRuleID(c.Params("ruleId"))
	if err := h.service.DeleteRule(c.Context(), ruleID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *GuardrailHandlers) ListViolations(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	violations, total, err := h.service.ListViolations(c.Context(), authCtx.TenantID, limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"violations": violations, "total": total})
}

// TestCheck allows testing guardrails against sample content without making an actual request.
func (h *GuardrailHandlers) TestCheck(c *fiber.Ctx) error {
	authCtx, ok := auth.GetAuthContext(c)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	var req struct {
		Messages []string `json:"messages"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	result, err := h.service.CheckMessages(c.Context(), authCtx.TenantID, req.Messages, "")
	if err != nil {
		return err
	}
	return c.JSON(result)
}
