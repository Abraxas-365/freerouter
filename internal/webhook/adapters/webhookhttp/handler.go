package webhookhttp

import (
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/gofiber/fiber/v2"
)

// Handler serves webhook subscription CRUD, delivery history, and test-fire endpoints.
type Handler struct {
	commands   webhook.Commands
	queries    webhook.Queries
	dispatcher webhook.Dispatcher
}

// New creates a webhook HTTP handler.
func New(commands webhook.Commands, queries webhook.Queries, dispatcher webhook.Dispatcher) *Handler {
	return &Handler{commands: commands, queries: queries, dispatcher: dispatcher}
}

// RegisterRoutes mounts webhook endpoints on the given group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/", server.RequirePermissions(server.PermWebhooksWrite), h.Create)
	r.Get("/", server.RequirePermissions(server.PermWebhooksRead), h.List)
	r.Get("/events", server.RequirePermissions(server.PermWebhooksRead), h.ListEvents)
	r.Get("/:id", server.RequirePermissions(server.PermWebhooksRead), h.Find)
	r.Patch("/:id", server.RequirePermissions(server.PermWebhooksWrite), h.Update)
	r.Delete("/:id", server.RequirePermissions(server.PermWebhooksWrite), h.Delete)
	r.Get("/:id/deliveries", server.RequirePermissions(server.PermWebhooksRead), h.ListDeliveries)
	r.Post("/:id/test", server.RequirePermissions(server.PermWebhooksWrite), h.Test)
}

// Create creates a new webhook subscription. The signing secret is returned
// only on this call — it is never exposed again.
func (h *Handler) Create(c *fiber.Ctx) error {
	var cmd webhook.CreateWebhook
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.Create(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         cfg.ID,
		"url":        cfg.URL,
		"secret":     cfg.Secret,
		"events":     cfg.Events,
		"enabled":    cfg.Enabled,
		"created_at": cfg.CreatedAt,
	})
}

// List returns paginated webhook subscriptions.
func (h *Handler) List(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)

	result, err := h.queries.List(c.Context(), page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// ListEvents returns all recognized webhook event types.
func (h *Handler) ListEvents(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"events": webhook.AllEvents()})
}

// Find returns a single webhook subscription by ID (secret omitted).
func (h *Handler) Find(c *fiber.Ctx) error {
	id, err := identity.ParseWebhookID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid webhook id")
	}

	cfg, err := h.queries.Find(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Update patches a webhook subscription.
func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := identity.ParseWebhookID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid webhook id")
	}

	var cmd webhook.UpdateWebhook
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.Update(c.Context(), id, cmd)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Delete removes a webhook subscription.
func (h *Handler) Delete(c *fiber.Ctx) error {
	id, err := identity.ParseWebhookID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid webhook id")
	}

	if err := h.commands.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListDeliveries returns paginated delivery attempts for a webhook.
func (h *Handler) ListDeliveries(c *fiber.Ctx) error {
	id, err := identity.ParseWebhookID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid webhook id")
	}

	// Ensure the webhook exists (404s cleanly instead of returning an empty page).
	if _, err := h.queries.Find(c.Context(), id); err != nil {
		return err
	}

	page := httpx.PaginationFromCtx(c)
	result, err := h.queries.ListDeliveries(c.Context(), id, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// Test fires a synthetic "webhook.test" event at the given webhook.
func (h *Handler) Test(c *fiber.Ctx) error {
	id, err := identity.ParseWebhookID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid webhook id")
	}

	if _, err := h.queries.Find(c.Context(), id); err != nil {
		return err
	}

	h.dispatcher.Fire("webhook.test", fiber.Map{
		"message":    "This is a test webhook event",
		"webhook_id": id.String(),
	})

	return c.JSON(fiber.Map{"message": "test event dispatched"})
}
