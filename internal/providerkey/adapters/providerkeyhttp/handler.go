package providerkeyhttp

import (
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/httpx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler exposes provider key CRUD over HTTP.
type Handler struct {
	cmd providerkey.Commands
	qry providerkey.Queries
}

// New creates a provider key HTTP handler.
func New(cmd providerkey.Commands, qry providerkey.Queries) *Handler {
	return &Handler{cmd: cmd, qry: qry}
}

// RegisterRoutes mounts provider key routes on the given router.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/", server.RequirePermissions(server.PermProviderKeysWrite), h.create)
	r.Get("/", server.RequirePermissions(server.PermProviderKeysRead), h.list)
	r.Get("/:id", server.RequirePermissions(server.PermProviderKeysRead), h.find)
	r.Put("/:id", server.RequirePermissions(server.PermProviderKeysWrite), h.update)
	r.Delete("/:id", server.RequirePermissions(server.PermProviderKeysWrite), h.delete)
}

func (h *Handler) create(c *fiber.Ctx) error {
	var input providerkey.Create
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	id, err := h.cmd.Create(c.Context(), input)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *Handler) find(c *fiber.Ctx) error {
	id, err := identity.ParseProviderKeyID(c.Params("id"))
	if err != nil {
		return err
	}
	k, err := h.qry.Find(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(k)
}

func (h *Handler) list(c *fiber.Ctx) error {
	page := httpx.PaginationFromCtx(c)
	filter := providerkey.Filter{}

	if providerID := c.Query("provider_id"); providerID != "" {
		id, err := identity.ParseProviderID(providerID)
		if err != nil {
			return err
		}
		filter.ProviderID = &id
	}
	if status := c.Query("status"); status != "" {
		s := providerkey.KeyStatus(status)
		filter.Status = &s
	}
	if keyType := c.Query("key_type"); keyType != "" {
		kt := providerkey.KeyType(keyType)
		filter.KeyType = &kt
	}

	result, err := h.qry.List(c.Context(), filter, page)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

func (h *Handler) update(c *fiber.Ctx) error {
	id, err := identity.ParseProviderKeyID(c.Params("id"))
	if err != nil {
		return err
	}
	var input providerkey.Update
	if err := c.BodyParser(&input); err != nil {
		return errx.Validation("invalid request body")
	}
	if err := h.cmd.Update(c.Context(), id, input); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) delete(c *fiber.Ctx) error {
	id, err := identity.ParseProviderKeyID(c.Params("id"))
	if err != nil {
		return err
	}
	if err := h.cmd.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
