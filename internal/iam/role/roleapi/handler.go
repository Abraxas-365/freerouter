package roleapi

import (
	"github.com/Abraxas-365/freerouter/internal/iam"
	"github.com/Abraxas-365/freerouter/internal/iam/auth"
	"github.com/Abraxas-365/freerouter/internal/iam/role"
	"github.com/Abraxas-365/freerouter/internal/iam/role/rolesrv"
	"github.com/Abraxas-365/freerouter/internal/iam/scopes"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/gofiber/fiber/v2"
)

type RoleHandlers struct {
	service *rolesrv.RoleService
}

func NewRoleHandlers(service *rolesrv.RoleService) *RoleHandlers {
	return &RoleHandlers{service: service}
}

func (h *RoleHandlers) RegisterRoutes(router fiber.Router, authMiddleware *auth.UnifiedAuthMiddleware) {
	roles := router.Group("/roles", authMiddleware.Authenticate())

	roles.Post("/", authMiddleware.RequireScope(scopes.ScopeRolesWrite), h.CreateRole)
	roles.Get("/", authMiddleware.RequireScope(scopes.ScopeRolesRead), h.GetTenantRoles)
	roles.Get("/:id", authMiddleware.RequireScope(scopes.ScopeRolesRead), h.GetRole)
	roles.Put("/:id", authMiddleware.RequireScope(scopes.ScopeRolesWrite), h.UpdateRole)
	roles.Delete("/:id", authMiddleware.RequireScope(scopes.ScopeRolesDelete), h.DeleteRole)

	// Role assignment
	roles.Post("/:id/assign", authMiddleware.RequireScope(scopes.ScopeRolesAssign), h.AssignRole)
	roles.Delete("/:id/users/:userId", authMiddleware.RequireScope(scopes.ScopeRolesAssign), h.UnassignRole)

	// User roles
	router.Get("/users/:userId/roles", authMiddleware.Authenticate(), authMiddleware.RequireScope(scopes.ScopeRolesRead), h.GetUserRoles)
}

func (h *RoleHandlers) CreateRole(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	req, err := kernel.BindAndValidate[role.CreateRoleRequest](c)
	if err != nil {
		return err
	}

	// Prevent privilege escalation: only grant scopes the caller holds
	if err := auth.EnsureCanGrantScopes(authContext, req.Scopes); err != nil {
		return err
	}

	r, err := h.service.CreateRole(c.Context(), authContext.TenantID, req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(r.ToDTO())
}

func (h *RoleHandlers) GetTenantRoles(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	response, err := h.service.GetTenantRoles(c.Context(), authContext.TenantID)
	if err != nil {
		return err
	}

	return c.JSON(response)
}

func (h *RoleHandlers) GetRole(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	roleID := c.Params("id")
	r, err := h.service.GetRoleByID(c.Context(), roleID, authContext.TenantID)
	if err != nil {
		return err
	}

	return c.JSON(r)
}

func (h *RoleHandlers) UpdateRole(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	roleID := c.Params("id")
	req, err := kernel.BindAndValidate[role.UpdateRoleRequest](c)
	if err != nil {
		return err
	}

	r, err := h.service.UpdateRole(c.Context(), roleID, authContext.TenantID, req)
	if err != nil {
		return err
	}

	return c.JSON(r)
}

func (h *RoleHandlers) DeleteRole(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	roleID := c.Params("id")
	if err := h.service.DeleteRole(c.Context(), roleID, authContext.TenantID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"message": "Role deleted successfully"})
}

func (h *RoleHandlers) AssignRole(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	roleID := c.Params("id")
	req, err := kernel.BindAndValidate[role.AssignRoleRequest](c)
	if err != nil {
		return err
	}

	// Prevent privilege escalation: assigning a role grants its scopes,
	// so the caller must hold every scope the role carries.
	r, err := h.service.GetRoleByID(c.Context(), roleID, authContext.TenantID)
	if err != nil {
		return err
	}
	if err := auth.EnsureCanGrantScopes(authContext, r.Scopes); err != nil {
		return err
	}

	if err := h.service.AssignRoleToUser(c.Context(), roleID, req.UserID, authContext.TenantID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"message": "Role assigned successfully"})
}

func (h *RoleHandlers) UnassignRole(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	roleID := c.Params("id")
	userID := kernel.UserID(c.Params("userId"))

	if err := h.service.UnassignRoleFromUser(c.Context(), roleID, userID, authContext.TenantID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{"message": "Role unassigned successfully"})
}

func (h *RoleHandlers) GetUserRoles(c *fiber.Ctx) error {
	authContext, ok := auth.GetAuthContext(c)
	if !ok {
		return iam.ErrUnauthorized()
	}

	userID := kernel.UserID(c.Params("userId"))
	response, err := h.service.GetUserRoles(c.Context(), userID, authContext.TenantID)
	if err != nil {
		return err
	}

	return c.JSON(response)
}
