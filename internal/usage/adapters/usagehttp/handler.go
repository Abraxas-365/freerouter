package usagehttp

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/Abraxas-365/freerouter/internal/usage"
	"github.com/gofiber/fiber/v2"
)

// Handler serves usage log and retention config endpoints.
type Handler struct {
	commands usage.Commands
	queries  usage.Queries
}

// New creates a usage HTTP handler.
func New(commands usage.Commands, queries usage.Queries) *Handler {
	return &Handler{commands: commands, queries: queries}
}

// RegisterRoutes mounts usage endpoints on the given router group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/", server.RequirePermissions(server.PermUsageRead), h.List)
	r.Get("/summary", server.RequirePermissions(server.PermUsageRead), h.Summary)
	r.Get("/retention", server.RequirePermissions(server.PermUsageRead), h.GetRetention)
	r.Put("/retention", server.RequirePermissions(server.PermUsageWrite), h.UpsertRetention)
	r.Delete("/retention", server.RequirePermissions(server.PermUsageWrite), h.DeleteRetention)
	r.Get("/:id", server.RequirePermissions(server.PermUsageRead), h.Find)
}

// List returns paginated usage logs with optional filters.
func (h *Handler) List(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)

	filter := usage.Filter{}
	if m := c.Query("model"); m != "" {
		filter.Model = &m
	}
	if p := c.Query("provider"); p != "" {
		filter.Provider = &p
	}
	if e := c.Query("has_error"); e == "true" || e == "false" {
		v := e == "true"
		filter.HasError = &v
	}
	if f := c.Query("from"); f != "" {
		t, err := time.Parse(time.RFC3339, f)
		if err != nil {
			return errx.Validation("invalid 'from' format, use RFC3339")
		}
		filter.From = &t
	}
	if t := c.Query("to"); t != "" {
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return errx.Validation("invalid 'to' format, use RFC3339")
		}
		filter.To = &parsed
	}

	result, err := h.queries.List(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// Find returns a single usage log by ID.
func (h *Handler) Find(c *fiber.Ctx) error {
	id, err := identity.ParseUsageLogID(c.Params("id"))
	if err != nil {
		return errx.Validation("invalid usage log id")
	}

	log, err := h.queries.Find(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(log)
}

// Summary returns aggregate usage statistics for a time period.
func (h *Handler) Summary(c *fiber.Ctx) error {
	var from, to *time.Time

	if f := c.Query("from"); f != "" {
		t, err := time.Parse(time.RFC3339, f)
		if err != nil {
			return errx.Validation("invalid 'from' format, use RFC3339")
		}
		from = &t
	}
	if t := c.Query("to"); t != "" {
		parsed, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return errx.Validation("invalid 'to' format, use RFC3339")
		}
		to = &parsed
	}

	resp, err := h.queries.GetSummary(c.Context(), from, to)
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// GetRetention returns the global usage-log data retention configuration.
func (h *Handler) GetRetention(c *fiber.Ctx) error {
	cfg, err := h.queries.GetRetention(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// UpsertRetention creates or updates the global data retention configuration.
func (h *Handler) UpsertRetention(c *fiber.Ctx) error {
	var cmd usage.UpsertRetention
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cfg, err := h.commands.UpsertRetention(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// DeleteRetention removes the global data retention configuration, reverting
// to "retain everything" (no purge worker action).
func (h *Handler) DeleteRetention(c *fiber.Ctx) error {
	if err := h.commands.DeleteRetention(c.Context()); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
