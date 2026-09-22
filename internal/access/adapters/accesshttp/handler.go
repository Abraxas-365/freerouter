package accesshttp

import (
	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
	"github.com/gofiber/fiber/v2"
)

// Handler serves user, role, and role-assignment management endpoints.
type Handler struct {
	userCmd   access.UserCommands
	userQ     access.UserQueries
	roleCmd   access.RoleCommands
	roleQ     access.RoleQueries
	assignCmd access.AssignmentCommands
	assignQ   access.AssignmentQueries
}

// New creates an access HTTP handler.
func New(
	userCmd access.UserCommands,
	userQ access.UserQueries,
	roleCmd access.RoleCommands,
	roleQ access.RoleQueries,
	assignCmd access.AssignmentCommands,
	assignQ access.AssignmentQueries,
) *Handler {
	return &Handler{
		userCmd:   userCmd,
		userQ:     userQ,
		roleCmd:   roleCmd,
		roleQ:     roleQ,
		assignCmd: assignCmd,
		assignQ:   assignQ,
	}
}

// RegisterRoutes mounts all access management routes on the given group.
// Expected mount point: /api/v1/access
func (h *Handler) RegisterRoutes(r fiber.Router) {
	// Users
	users := r.Group("/users")
	users.Post("/", server.RequirePermissions(server.PermUsersWrite), h.CreateUser)
	users.Get("/", server.RequirePermissions(server.PermUsersRead), h.ListUsers)
	users.Get("/:id", server.RequirePermissions(server.PermUsersRead), h.FindUser)
	users.Patch("/:id", server.RequirePermissions(server.PermUsersWrite), h.UpdateUser)
	users.Delete("/:id", server.RequirePermissions(server.PermUsersWrite), h.SuspendUser)

	// Roles
	roles := r.Group("/roles")
	roles.Post("/", server.RequirePermissions(server.PermRolesWrite), h.CreateRole)
	roles.Get("/", server.RequirePermissions(server.PermRolesRead), h.ListRoles)
	roles.Put("/:id", server.RequirePermissions(server.PermRolesWrite), h.UpdateRole)
	roles.Delete("/:id", server.RequirePermissions(server.PermRolesWrite), h.DeleteRole)

	// Role assignments
	assignments := r.Group("/role-assignments")
	assignments.Post("/", server.RequirePermissions(server.PermRolesWrite), h.AssignRole)
	assignments.Get("/", server.RequirePermissions(server.PermRolesRead), h.ListAssignments)
	assignments.Delete("/", server.RequirePermissions(server.PermRolesWrite), h.UnassignRole)
}

// ── Users ───────────────────────────────────────────────────────────

func (h *Handler) CreateUser(c *fiber.Ctx) error {
	var cmd access.CreateUser
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	user, err := h.userCmd.CreateUser(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}

func (h *Handler) ListUsers(c *fiber.Ctx) error {
	users, err := h.userQ.ListUsers(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(users)
}

func (h *Handler) FindUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errx.Validation("id is required")
	}
	user, err := h.userQ.FindUser(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(user)
}

func (h *Handler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errx.Validation("id is required")
	}
	var cmd access.UpdateUser
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	if err := h.userCmd.UpdateUser(c.Context(), id, cmd); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) SuspendUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errx.Validation("id is required")
	}
	if err := h.userCmd.SuspendUser(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ── Roles ───────────────────────────────────────────────────────────

func (h *Handler) CreateRole(c *fiber.Ctx) error {
	var cmd access.CreateRole
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	role, err := h.roleCmd.CreateRole(c.Context(), cmd)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(role)
}

func (h *Handler) ListRoles(c *fiber.Ctx) error {
	roles, err := h.roleQ.ListRoles(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(roles)
}

func (h *Handler) UpdateRole(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errx.Validation("id is required")
	}
	var cmd access.UpdateRole
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	if err := h.roleCmd.UpdateRole(c.Context(), id, cmd); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) DeleteRole(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errx.Validation("id is required")
	}
	if err := h.roleCmd.DeleteRole(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ── Role Assignments ────────────────────────────────────────────────

func (h *Handler) AssignRole(c *fiber.Ctx) error {
	var cmd access.AssignRole
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	if err := h.assignCmd.AssignRole(c.Context(), cmd); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListAssignments(c *fiber.Ctx) error {
	assignments, err := h.assignQ.ListAssignments(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(assignments)
}

func (h *Handler) UnassignRole(c *fiber.Ctx) error {
	var cmd access.AssignRole
	if err := c.BodyParser(&cmd); err != nil {
		return errx.Validation("invalid request body: " + err.Error())
	}
	if err := h.assignCmd.UnassignRole(c.Context(), cmd); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
