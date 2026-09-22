package access

import "context"

// ── User interfaces ─────────────────────────────────────────────────

// UserCommands defines write operations for users.
type UserCommands interface {
	CreateUser(ctx context.Context, input CreateUser) (User, error)
	UpdateUser(ctx context.Context, id string, input UpdateUser) error
	SuspendUser(ctx context.Context, id string) error
}

// UserQueries defines read operations for users.
type UserQueries interface {
	ListUsers(ctx context.Context) ([]User, error)
	FindUser(ctx context.Context, id string) (User, error)
}

// ── Role interfaces ─────────────────────────────────────────────────

// RoleCommands defines write operations for roles.
type RoleCommands interface {
	CreateRole(ctx context.Context, input CreateRole) (Role, error)
	UpdateRole(ctx context.Context, id string, input UpdateRole) error
	DeleteRole(ctx context.Context, id string) error
}

// RoleQueries defines read operations for roles.
type RoleQueries interface {
	ListRoles(ctx context.Context) ([]Role, error)
}

// ── Assignment interfaces ───────────────────────────────────────────

// AssignmentCommands defines write operations for role assignments.
type AssignmentCommands interface {
	AssignRole(ctx context.Context, input AssignRole) error
	UnassignRole(ctx context.Context, input AssignRole) error
}

// AssignmentQueries defines read operations for role assignments.
type AssignmentQueries interface {
	ListAssignments(ctx context.Context) ([]RoleAssignment, error)
}

// ── Store (IAMKit adapter) ──────────────────────────────────────────

// Store abstracts all IAMKit access management for users, roles, and assignments.
type Store interface {
	// Users
	CreateUser(ctx context.Context, input CreateUser) (User, error)
	UpdateUser(ctx context.Context, id string, input UpdateUser) error
	SuspendUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]User, error)
	FindUser(ctx context.Context, id string) (User, error)

	// Roles
	CreateRole(ctx context.Context, input CreateRole, resourceID string) (Role, error)
	UpdateRole(ctx context.Context, id string, input UpdateRole, resourceID string) error
	DeleteRole(ctx context.Context, id string) error
	ListRoles(ctx context.Context) ([]Role, error)

	// Assignments
	AssignRole(ctx context.Context, input AssignRole, organizationID string) error
	UnassignRole(ctx context.Context, input AssignRole, organizationID string) error
	ListAssignments(ctx context.Context) ([]RoleAssignment, error)
}
