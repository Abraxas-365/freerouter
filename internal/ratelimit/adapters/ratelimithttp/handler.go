package ratelimithttp

import (
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/ratelimit"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler serves rate limit config CRUD endpoints.
type Handler struct {
	commands ratelimit.Commands
	queries  ratelimit.Queries
}

// New creates a rate limit HTTP handler.
func New(commands ratelimit.Commands, queries ratelimit.Queries) *Handler {
	return &Handler{commands: commands, queries: queries}
}

// RegisterRoutes mounts rate limit config endpoints on the given group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/", server.RequirePermissions(server.PermRateLimitsWrite), h.Create)
	r.Get("/", server.RequirePermissions(server.PermRateLimitsRead), h.List)
	r.Get("/:id", server.RequirePermissions(server.PermRateLimitsRead), h.Find)
	r.Patch("/:id", server.RequirePermissions(server.PermRateLimitsWrite), h.Update)
	r.Delete("/:id", server.RequirePermissions(server.PermRateLimitsWrite), h.Delete)
}

// Create creates a new rate limit config.
func (h *Handler) Create(c *fiber.Ctx) error {
	var cmd ratelimit.CreateRateLimitConfig
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.Create(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(cfg)
}

// List returns paginated rate limit configs with optional filters.
func (h *Handler) List(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)

	filter := ratelimit.Filter{}
	if s := c.Query("subject_id"); s != "" {
		filter.SubjectID = &s
	}
	if n := c.Query("name"); n != "" {
		filter.Name = &n
	}

	result, err := h.queries.List(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// Find returns a single rate limit config by ID.
func (h *Handler) Find(c *fiber.Ctx) error {
	id, err := identity.ParseRateLimitConfigID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid rate limit config id")
	}

	cfg, err := h.queries.Find(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Update patches a rate limit config.
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := identity.ParseRateLimitConfigID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid rate limit config id")
	}

	var cmd ratelimit.UpdateRateLimitConfig
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.Update(c.Context(), id, cmd)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Delete removes a rate limit config.
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := identity.ParseRateLimitConfigID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid rate limit config id")
	}

	if err := h.commands.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
