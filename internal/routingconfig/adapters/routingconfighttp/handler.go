package routingconfighttp

import (
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/routingconfig"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler serves routing config CRUD endpoints.
type Handler struct {
	commands routingconfig.Commands
	queries  routingconfig.Queries
}

// New creates a routing config HTTP handler.
func New(commands routingconfig.Commands, queries routingconfig.Queries) *Handler {
	return &Handler{commands: commands, queries: queries}
}

// RegisterRoutes mounts routing config endpoints on the given group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/", server.RequirePermissions(server.PermRoutingWrite), h.Create)
	r.Get("/", server.RequirePermissions(server.PermRoutingRead), h.List)
	r.Get("/:id", server.RequirePermissions(server.PermRoutingRead), h.Find)
	r.Patch("/:id", server.RequirePermissions(server.PermRoutingWrite), h.Update)
	r.Delete("/:id", server.RequirePermissions(server.PermRoutingWrite), h.Delete)
}

// Create creates a new routing config.
func (h *Handler) Create(c *fiber.Ctx) error {
	var cmd routingconfig.CreateRoutingConfig
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.Create(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(cfg)
}

// List returns paginated routing configs with optional filters.
func (h *Handler) List(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)

	filter := routingconfig.Filter{}
	if s := c.Query("subject_id"); s != "" {
		filter.SubjectID = &s
	}

	result, err := h.queries.List(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// Find returns a single routing config by ID.
func (h *Handler) Find(c *fiber.Ctx) error {
	id, err := identity.ParseRoutingConfigID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid routing config id")
	}

	cfg, err := h.queries.Find(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Update patches a routing config.
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := identity.ParseRoutingConfigID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid routing config id")
	}

	var cmd routingconfig.UpdateRoutingConfig
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.Update(c.Context(), id, cmd)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Delete removes a routing config.
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := identity.ParseRoutingConfigID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid routing config id")
	}

	if err := h.commands.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
