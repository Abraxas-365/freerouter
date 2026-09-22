package apikeyhttp

import (
	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler serves service account management endpoints.
type Handler struct {
	commands apikey.Commands
	queries  apikey.Queries
}

// New creates a service account HTTP handler.
func New(commands apikey.Commands, queries apikey.Queries) *Handler {
	return &Handler{commands: commands, queries: queries}
}

// RegisterRoutes mounts service account endpoints on the given group.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/", server.RequirePermissions(server.PermServiceAccountsWrite), h.Create)
	r.Get("/", server.RequirePermissions(server.PermServiceAccountsRead), h.List)
	r.Get("/applications", server.RequirePermissions(server.PermServiceAccountsRead), h.ListApplications)
	r.Delete("/:id", server.RequirePermissions(server.PermServiceAccountsWrite), h.Revoke)
}

// Create creates a new service account, returning the one-time secret.
func (h *Handler) Create(c *fiber.Ctx) error {
	var cmd apikey.CreateServiceAccount
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}

	cred, err := h.commands.Create(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(cred)
}

// List returns all service accounts (secrets are not included).
func (h *Handler) List(c *fiber.Ctx) error {
	accounts, err := h.queries.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(accounts)
}

// ListApplications returns the IAMKit applications a new service account can
// be created against (i.e. "which service is this credential for").
func (h *Handler) ListApplications(c *fiber.Ctx) error {
	apps, err := h.queries.ListApplications(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(apps)
}

// Revoke permanently revokes a service account.
func (h *Handler) Revoke(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errx.Validation("id is required")
	}

	if err := h.commands.Revoke(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
