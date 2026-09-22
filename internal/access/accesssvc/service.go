package accesssvc

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/errx"
)

// Service implements user, role, and assignment commands/queries.
// It validates input and delegates to the IAMKit-backed store,
// injecting FreeRouter's IAMKit resource and organization IDs.
type Service struct {
	store          access.Store
	resourceID     string
	organizationID string
}

// New creates a new access service.
func New(store access.Store, resourceID, organizationID string) *Service {
	return &Service{
		store:          store,
		resourceID:     resourceID,
		organizationID: organizationID,
	}
}

// ── Users ───────────────────────────────────────────────────────────

func (s *Service) CreateUser(ctx context.Context, input access.CreateUser) (access.User, error) {
	if err := input.Validate(); err != nil {
		return access.User{}, err
	}
	return s.store.CreateUser(ctx, input)
}

func (s *Service) UpdateUser(ctx context.Context, id string, input access.UpdateUser) error {
	if id == "" {
		return errx.Validation("user id is required")
	}
	return s.store.UpdateUser(ctx, id, input)
}

func (s *Service) SuspendUser(ctx context.Context, id string) error {
	if id == "" {
		return errx.Validation("user id is required")
	}
	return s.store.SuspendUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context) ([]access.User, error) {
	return s.store.ListUsers(ctx)
}

func (s *Service) FindUser(ctx context.Context, id string) (access.User, error) {
	if id == "" {
		return access.User{}, errx.Validation("user id is required")
	}
	return s.store.FindUser(ctx, id)
}

// ── Roles ───────────────────────────────────────────────────────────

func (s *Service) CreateRole(ctx context.Context, input access.CreateRole) (access.Role, error) {
	if err := input.Validate(); err != nil {
		return access.Role{}, err
	}
	return s.store.CreateRole(ctx, input, s.resourceID)
}

func (s *Service) UpdateRole(ctx context.Context, id string, input access.UpdateRole) error {
	if id == "" {
		return errx.Validation("role id is required")
	}
	if err := input.Validate(); err != nil {
		return err
	}
	return s.store.UpdateRole(ctx, id, input, s.resourceID)
}

func (s *Service) DeleteRole(ctx context.Context, id string) error {
	if id == "" {
		return errx.Validation("role id is required")
	}
	return s.store.DeleteRole(ctx, id)
}

func (s *Service) ListRoles(ctx context.Context) ([]access.Role, error) {
	return s.store.ListRoles(ctx)
}

// ── Assignments ─────────────────────────────────────────────────────

func (s *Service) AssignRole(ctx context.Context, input access.AssignRole) error {
	if err := input.Validate(); err != nil {
		return err
	}
	return s.store.AssignRole(ctx, input, s.organizationID)
}

func (s *Service) UnassignRole(ctx context.Context, input access.AssignRole) error {
	if err := input.Validate(); err != nil {
		return err
	}
	return s.store.UnassignRole(ctx, input, s.organizationID)
}

func (s *Service) ListAssignments(ctx context.Context) ([]access.RoleAssignment, error) {
	return s.store.ListAssignments(ctx)
}

var _ access.UserCommands = (*Service)(nil)
var _ access.UserQueries = (*Service)(nil)
var _ access.RoleCommands = (*Service)(nil)
var _ access.RoleQueries = (*Service)(nil)
var _ access.AssignmentCommands = (*Service)(nil)
var _ access.AssignmentQueries = (*Service)(nil)
