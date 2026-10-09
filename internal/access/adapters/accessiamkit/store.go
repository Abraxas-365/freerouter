// Package accessiamkit implements access.Store against IAMKit's
// permission-scoped API (/api/v1) as FreeRouter's backend service account
// (iam:users:*, iam:roles:*).
package accessiamkit

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/iamx"
	"github.com/Abraxas-365/iamkit/sdk/apiclient"
)

// Store implements access.Store using IAMKit.
type Store struct {
	iam *iamx.Client
}

// New creates an IAMKit-backed access store.
func New(iam *iamx.Client) *Store {
	return &Store{iam: iam}
}

// ── Users ───────────────────────────────────────────────────────────

// CreateUser creates the user with organizationID as home organization,
// which also makes them a member there (roles need a membership).
func (s *Store) CreateUser(ctx context.Context, input access.CreateUser, organizationID string) (access.User, error) {
	var created apiclient.Created
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		created, err = env.CreateUser(ctx, apiclient.CreateUser{
			Email:              input.Email,
			Name:               input.Name,
			Password:           input.Password,
			HomeOrganizationID: organizationID,
		})
		return err
	})
	if err != nil {
		return access.User{}, iamx.Translate(err, "create user")
	}
	return access.User{
		ID:                 created.ID,
		Email:              input.Email,
		Name:               input.Name,
		Active:             true,
		HomeOrganizationID: organizationID,
	}, nil
}

func (s *Store) UpdateUser(ctx context.Context, id string, input access.UpdateUser) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.UpdateUser(ctx, id, apiclient.UserPatch{Name: input.Name, Active: input.Active})
	})
	return iamx.Translate(err, "update user")
}

func (s *Store) SuspendUser(ctx context.Context, id string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.SuspendUser(ctx, id)
	})
	return iamx.Translate(err, "suspend user")
}

func (s *Store) ListUsers(ctx context.Context) ([]access.User, error) {
	var users []apiclient.User
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		users, err = env.Users(ctx)
		return err
	})
	if err != nil {
		return nil, iamx.Translate(err, "list users")
	}
	out := make([]access.User, len(users))
	for i, u := range users {
		out[i] = toUser(u)
	}
	return out, nil
}

func (s *Store) FindUser(ctx context.Context, id string) (access.User, error) {
	var u apiclient.User
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		u, err = env.User(ctx, id)
		return err
	})
	if err != nil {
		return access.User{}, iamx.Translate(err, "find user")
	}
	return toUser(u), nil
}

func (s *Store) RevokeSessions(ctx context.Context, id string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		_, err := env.RevokeUserSessions(ctx, id)
		return err
	})
	return iamx.Translate(err, "revoke user sessions")
}

func toUser(u apiclient.User) access.User {
	return access.User{ID: u.ID, Email: u.Email, Name: u.Name, Active: u.Active, HomeOrganizationID: u.HomeOrganizationID}
}

// ── Roles ───────────────────────────────────────────────────────────

func (s *Store) CreateRole(ctx context.Context, input access.CreateRole, resourceID string) (access.Role, error) {
	var created apiclient.Created
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		created, err = env.CreateRole(ctx, apiclient.Role{
			Name:        input.Name,
			ResourceID:  resourceID,
			Permissions: input.Permissions,
		})
		return err
	})
	if err != nil {
		return access.Role{}, iamx.Translate(err, "create role")
	}
	return access.Role{
		ID:          created.ID,
		Name:        input.Name,
		ResourceID:  resourceID,
		Permissions: input.Permissions,
	}, nil
}

func (s *Store) UpdateRole(ctx context.Context, id string, input access.UpdateRole, resourceID string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.UpdateRole(ctx, id, apiclient.Role{
			Name:        input.Name,
			ResourceID:  resourceID,
			Permissions: input.Permissions,
		})
	})
	return iamx.Translate(err, "update role")
}

func (s *Store) DeleteRole(ctx context.Context, id string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.DeleteRole(ctx, id)
	})
	return iamx.Translate(err, "delete role")
}

func (s *Store) FindRole(ctx context.Context, id string) (access.Role, error) {
	var r apiclient.Role
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		r, err = env.Role(ctx, id)
		return err
	})
	if err != nil {
		return access.Role{}, iamx.Translate(err, "find role")
	}
	return toRole(r), nil
}

func (s *Store) ListRoles(ctx context.Context) ([]access.Role, error) {
	var roles []apiclient.Role
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		roles, err = env.Roles(ctx)
		return err
	})
	if err != nil {
		return nil, iamx.Translate(err, "list roles")
	}
	out := make([]access.Role, len(roles))
	for i, r := range roles {
		out[i] = toRole(r)
	}
	return out, nil
}

func toRole(r apiclient.Role) access.Role {
	return access.Role{ID: r.ID, Name: r.Name, ResourceID: r.ResourceID, Permissions: r.Permissions}
}

// ── Assignments ─────────────────────────────────────────────────────

func (s *Store) AssignRole(ctx context.Context, input access.AssignRole, organizationID string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.AssignRole(ctx, apiclient.RoleAssignment{
			OrganizationID: organizationID,
			UserID:         input.UserID,
			RoleID:         input.RoleID,
		})
	})
	return iamx.Translate(err, "assign role")
}

func (s *Store) UnassignRole(ctx context.Context, input access.AssignRole, organizationID string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.UnassignRole(ctx, apiclient.RoleAssignment{
			OrganizationID: organizationID,
			UserID:         input.UserID,
			RoleID:         input.RoleID,
		})
	})
	return iamx.Translate(err, "unassign role")
}

func (s *Store) ListAssignments(ctx context.Context) ([]access.RoleAssignment, error) {
	var views []apiclient.RoleAssignmentView
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		views, err = env.RoleAssignments(ctx)
		return err
	})
	if err != nil {
		return nil, iamx.Translate(err, "list role assignments")
	}
	out := make([]access.RoleAssignment, len(views))
	for i, a := range views {
		out[i] = access.RoleAssignment{
			UserID:         a.UserID,
			RoleID:         a.RoleID,
			OrganizationID: a.OrganizationID,
			ResourceID:     a.ResourceID,
		}
	}
	return out, nil
}

var _ access.Store = (*Store)(nil)
