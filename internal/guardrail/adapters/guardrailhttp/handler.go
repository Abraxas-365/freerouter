package guardrailhttp

import (
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler serves guardrail config, custom rule, and violation endpoints.
type Handler struct {
	commands guardrail.Commands
	queries  guardrail.Queries
}

// New creates a guardrail HTTP handler.
func New(commands guardrail.Commands, queries guardrail.Queries) *Handler {
	return &Handler{commands: commands, queries: queries}
}

// RegisterRoutes mounts guardrail endpoints on the given group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/config", server.RequirePermissions(server.PermGuardrailsRead), h.GetConfig)
	r.Put("/config", server.RequirePermissions(server.PermGuardrailsWrite), h.UpsertConfig)

	r.Get("/rules", server.RequirePermissions(server.PermGuardrailsRead), h.ListRules)
	r.Post("/rules", server.RequirePermissions(server.PermGuardrailsWrite), h.CreateRule)
	r.Get("/rules/:id", server.RequirePermissions(server.PermGuardrailsRead), h.FindRule)
	r.Patch("/rules/:id", server.RequirePermissions(server.PermGuardrailsWrite), h.UpdateRule)
	r.Delete("/rules/:id", server.RequirePermissions(server.PermGuardrailsWrite), h.DeleteRule)

	r.Get("/violations", server.RequirePermissions(server.PermGuardrailsRead), h.ListViolations)
}

// GetConfig returns the global guardrail configuration.
func (h *Handler) GetConfig(c *fiber.Ctx) error {
	cfg, err := h.queries.GetConfig(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// UpsertConfig creates or updates the global guardrail configuration.
func (h *Handler) UpsertConfig(c *fiber.Ctx) error {
	var cmd guardrail.UpsertConfig
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.UpsertConfig(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// ListRules returns all custom guardrail rules.
func (h *Handler) ListRules(c *fiber.Ctx) error {
	rules, err := h.queries.ListRules(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(rules)
}

// FindRule returns a single custom rule by ID.
func (h *Handler) FindRule(c *fiber.Ctx) error {
	id, err := identity.ParseGuardrailRuleID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid guardrail rule id")
	}

	rule, err := h.queries.FindRule(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(rule)
}

// CreateRule creates a new custom guardrail rule.
func (h *Handler) CreateRule(c *fiber.Ctx) error {
	var cmd guardrail.CreateRule
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	rule, err := h.commands.CreateRule(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(rule)
}

// UpdateRule patches a custom guardrail rule.
func (h *Handler) UpdateRule(c *fiber.Ctx) error {
	id, err := identity.ParseGuardrailRuleID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid guardrail rule id")
	}

	var cmd guardrail.UpdateRule
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	rule, err := h.commands.UpdateRule(c.Context(), id, cmd)
	if err != nil {
		return err
	}
	return c.JSON(rule)
}

// DeleteRule removes a custom guardrail rule.
func (h *Handler) DeleteRule(c *fiber.Ctx) error {
	id, err := identity.ParseGuardrailRuleID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid guardrail rule id")
	}

	if err := h.commands.DeleteRule(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListViolations returns paginated logged guardrail violations.
func (h *Handler) ListViolations(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)

	result, err := h.queries.ListViolations(c.Context(), page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
