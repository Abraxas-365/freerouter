package accessiamkit

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/iamkit/sdk/iamclient"
)

// Store implements access.Store using the IAMKit management SDK.
type Store struct {
	env iamclient.Environment
}

// New creates an IAMKit-backed access store.
func New(client *iamclient.Client, environmentID string) *Store {
	return &Store{env: client.Environment(environmentID)}
}

// ── Users ───────────────────────────────────────────────────────────

func (s *Store) CreateUser(ctx context.Context, input access.CreateUser) (access.User, error) {
	created, err := s.env.CreateUser(ctx, iamclient.CreateUser{
		Email:    input.Email,
		Name:     input.Name,
		Password: input.Password,
	})
	if err != nil {
		return access.User{}, errx.Wrap(err, "iamkit: create user", errx.TypeInternal)
	}
	return access.User{
		ID:     created.ID,
		Email:  input.Email,
		Name:   input.Name,
		Active: true,
	}, nil
}

func (s *Store) UpdateUser(ctx context.Context, id string, input access.UpdateUser) error {
	patch := iamclient.UserPatch{
		Name:   input.Name,
		Active: input.Active,
	}
	if err := s.env.UpdateUser(ctx, id, patch); err != nil {
		return errx.Wrap(err, "iamkit: update user", errx.TypeInternal)
	}
	return nil
}

func (s *Store) SuspendUser(ctx context.Context, id string) error {
	if err := s.env.SuspendUser(ctx, id); err != nil {
		return errx.Wrap(err, "iamkit: suspend user", errx.TypeInternal)
	}
	return nil
}

func (s *Store) ListUsers(ctx context.Context) ([]access.User, error) {
	users, err := s.env.Users(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "iamkit: list users", errx.TypeInternal)
	}
	out := make([]access.User, len(users))
	for i, u := range users {
		out[i] = access.User{
			ID:     u.ID,
			Email:  u.Email,
			Name:   u.Name,
			Active: u.Active,
		}
	}
	return out, nil
}

func (s *Store) FindUser(ctx context.Context, id string) (access.User, error) {
	u, err := s.env.User(ctx, id)
	if err != nil {
		return access.User{}, errx.Wrap(err, "iamkit: find user", errx.TypeInternal)
	}
	return access.User{
		ID:     u.ID,
		Email:  u.Email,
		Name:   u.Name,
		Active: u.Active,
	}, nil
}

// ── Roles ───────────────────────────────────────────────────────────

func (s *Store) CreateRole(ctx context.Context, input access.CreateRole, resourceID string) (access.Role, error) {
	created, err := s.env.CreateRole(ctx, iamclient.Role{
		Name:        input.Name,
		ResourceID:  resourceID,
		Permissions: input.Permissions,
	})
	if err != nil {
		return access.Role{}, errx.Wrap(err, "iamkit: create role", errx.TypeInternal)
	}
	return access.Role{
		ID:          created.ID,
		Name:        input.Name,
		Permissions: input.Permissions,
	}, nil
}

func (s *Store) UpdateRole(ctx context.Context, id string, input access.UpdateRole, resourceID string) error {
	if err := s.env.UpdateRole(ctx, id, iamclient.Role{
		Name:        input.Name,
		ResourceID:  resourceID,
		Permissions: input.Permissions,
	}); err != nil {
		return errx.Wrap(err, "iamkit: update role", errx.TypeInternal)
	}
	return nil
}

func (s *Store) DeleteRole(ctx context.Context, id string) error {
	if err := s.env.DeleteRole(ctx, id); err != nil {
		return errx.Wrap(err, "iamkit: delete role", errx.TypeInternal)
	}
	return nil
}

func (s *Store) ListRoles(ctx context.Context) ([]access.Role, error) {
	roles, err := s.env.Roles(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "iamkit: list roles", errx.TypeInternal)
	}
	out := make([]access.Role, len(roles))
	for i, r := range roles {
		out[i] = access.Role{
			ID:          r.ID,
			Name:        r.Name,
			Permissions: r.Permissions,
		}
	}
	return out, nil
}

// ── Assignments ─────────────────────────────────────────────────────

func (s *Store) AssignRole(ctx context.Context, input access.AssignRole, organizationID string) error {
	if err := s.env.AssignRole(ctx, iamclient.RoleAssignment{
		OrganizationID: organizationID,
		UserID:         input.UserID,
		RoleID:         input.RoleID,
	}); err != nil {
		return errx.Wrap(err, "iamkit: assign role", errx.TypeInternal)
	}
	return nil
}

func (s *Store) UnassignRole(ctx context.Context, input access.AssignRole, organizationID string) error {
	if err := s.env.UnassignRole(ctx, iamclient.RoleAssignment{
		OrganizationID: organizationID,
		UserID:         input.UserID,
		RoleID:         input.RoleID,
	}); err != nil {
		return errx.Wrap(err, "iamkit: unassign role", errx.TypeInternal)
	}
	return nil
}

func (s *Store) ListAssignments(ctx context.Context) ([]access.RoleAssignment, error) {
	assignments, err := s.env.RoleAssignments(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "iamkit: list role assignments", errx.TypeInternal)
	}
	out := make([]access.RoleAssignment, len(assignments))
	for i, a := range assignments {
		out[i] = access.RoleAssignment{
			UserID:         a.UserID,
			RoleID:         a.RoleID,
			OrganizationID: a.OrganizationID,
		}
	}
	return out, nil
}

var _ access.Store = (*Store)(nil)
