// Package access manages FreeRouter users, roles, and role assignments.
// All data is stored in IAMKit — FreeRouter exposes a scoped admin API
// so normal admins never need direct IAMKit access.
package access

import (
	"fmt"
	"strings"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
)

// ── User ────────────────────────────────────────────────────────────

// User represents a FreeRouter user (backed by an IAMKit identity).
type User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// CreateUser is the input for registering a new user.
type CreateUser struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// Validate checks required fields for user creation.
func (c CreateUser) Validate() error {
	if strings.TrimSpace(c.Email) == "" {
		return errx.Validation("email is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errx.Validation("name is required")
	}
	if len(c.Password) < 8 {
		return errx.Validation("password must be at least 8 characters")
	}
	return nil
}

// UpdateUser is the input for patching a user.
type UpdateUser struct {
	Name   *string `json:"name,omitempty"`
	Active *bool   `json:"active,omitempty"`
}

// ── Role ────────────────────────────────────────────────────────────

// Role groups a set of FreeRouter permissions under a name.
// Backed by an IAMKit role bound to the FreeRouter resource.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// CreateRole is the input for creating a new role.
type CreateRole struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// Validate checks the create role request.
func (c CreateRole) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errx.Validation("name is required")
	}
	if len(c.Permissions) == 0 {
		return errx.Validation("at least one permission is required")
	}
	for _, p := range c.Permissions {
		if !server.ValidPermissions[p] {
			return errx.Validation(fmt.Sprintf("unknown permission: %q", p))
		}
	}
	return nil
}

// UpdateRole is the input for updating an existing role.
type UpdateRole struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// Validate checks the update role request.
func (u UpdateRole) Validate() error {
	if strings.TrimSpace(u.Name) == "" {
		return errx.Validation("name is required")
	}
	if len(u.Permissions) == 0 {
		return errx.Validation("at least one permission is required")
	}
	for _, p := range u.Permissions {
		if !server.ValidPermissions[p] {
			return errx.Validation(fmt.Sprintf("unknown permission: %q", p))
		}
	}
	return nil
}

// ── Role Assignment ─────────────────────────────────────────────────

// RoleAssignment binds a user to a role within an organization.
type RoleAssignment struct {
	UserID         string `json:"user_id"`
	RoleID         string `json:"role_id"`
	OrganizationID string `json:"organization_id"`
}

// AssignRole is the input for assigning a role to a user.
type AssignRole struct {
	UserID string `json:"user_id"`
	RoleID string `json:"role_id"`
}

// Validate checks the assign role request.
func (a AssignRole) Validate() error {
	if strings.TrimSpace(a.UserID) == "" {
		return errx.Validation("user_id is required")
	}
	if strings.TrimSpace(a.RoleID) == "" {
		return errx.Validation("role_id is required")
	}
	return nil
}
