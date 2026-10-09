package accesssvc

import (
	"context"
	"log/slog"

	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/errx"
)

// Service implements user, role, and assignment commands/queries.
// It validates input and delegates to the IAMKit-backed store,
// injecting FreeRouter's IAMKit resource and organization IDs.
//
// The backend IAMKit credential's iam:users:* and iam:roles:* permissions
// are environment-wide, so this service is the boundary: only users homed
// in FreeRouter's organization and roles on FreeRouter's resource are
// listed, changed, deleted or assigned. A FreeRouter admin can never see or
// touch another organization's users, nor hand out roles of another
// resource (for example the IAM resource's administration roles).
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
	return s.store.CreateUser(ctx, input, s.organizationID)
}

func (s *Service) UpdateUser(ctx context.Context, id string, input access.UpdateUser) error {
	if err := s.ownUser(ctx, id); err != nil {
		return err
	}
	return s.store.UpdateUser(ctx, id, input)
}

func (s *Service) SuspendUser(ctx context.Context, id string) error {
	if err := s.ownUser(ctx, id); err != nil {
		return err
	}
	return s.store.SuspendUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context) ([]access.User, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]access.User, 0, len(users))
	for _, u := range users {
		if u.HomeOrganizationID == s.organizationID {
			out = append(out, u)
		}
	}
	return out, nil
}

func (s *Service) FindUser(ctx context.Context, id string) (access.User, error) {
	if id == "" {
		return access.User{}, errx.Validation("user id is required")
	}
	u, err := s.store.FindUser(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return access.User{}, errUserNotFound()
		}
		return access.User{}, err
	}
	if u.HomeOrganizationID != s.organizationID {
		return access.User{}, errUserNotFound()
	}
	return u, nil
}

// Foreign and nonexistent users/roles get the same error, so callers cannot
// probe which ids exist outside FreeRouter's boundary.
func errUserNotFound() error { return errx.NotFound("user not found") }
func errRoleNotFound() error { return errx.NotFound("role not found") }

func isNotFound(err error) bool {
	var x *errx.Error
	return errx.As(err, &x) && x.Type == errx.TypeNotFound
}

// ownUser succeeds only for a user homed in FreeRouter's organization; any
// other user is reported as not found. The backend credential's
// iam:users:* permissions are environment-wide, so this is the boundary.
func (s *Service) ownUser(ctx context.Context, id string) error {
	_, err := s.FindUser(ctx, id)
	return err
}

// ── Roles ───────────────────────────────────────────────────────────

func (s *Service) CreateRole(ctx context.Context, input access.CreateRole) (access.Role, error) {
	if err := input.Validate(); err != nil {
		return access.Role{}, err
	}
	return s.store.CreateRole(ctx, input, s.resourceID)
}

func (s *Service) UpdateRole(ctx context.Context, id string, input access.UpdateRole) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.ownRole(ctx, id); err != nil {
		return err
	}
	return s.store.UpdateRole(ctx, id, input, s.resourceID)
}

func (s *Service) DeleteRole(ctx context.Context, id string) error {
	if err := s.ownRole(ctx, id); err != nil {
		return err
	}
	return s.store.DeleteRole(ctx, id)
}

func (s *Service) ListRoles(ctx context.Context) ([]access.Role, error) {
	roles, err := s.store.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]access.Role, 0, len(roles))
	for _, r := range roles {
		if r.ResourceID == s.resourceID {
			out = append(out, r)
		}
	}
	return out, nil
}

// ownRole succeeds only for a role on FreeRouter's resource; any other role
// is reported as not found.
func (s *Service) ownRole(ctx context.Context, id string) error {
	if id == "" {
		return errx.Validation("role id is required")
	}
	role, err := s.store.FindRole(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return errRoleNotFound()
		}
		return err
	}
	if role.ResourceID != s.resourceID {
		return errRoleNotFound()
	}
	return nil
}

// ── Assignments ─────────────────────────────────────────────────────

func (s *Service) AssignRole(ctx context.Context, input access.AssignRole) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.ownRole(ctx, input.RoleID); err != nil {
		return err
	}
	if err := s.ownUser(ctx, input.UserID); err != nil {
		return err
	}
	return s.store.AssignRole(ctx, input, s.organizationID)
}

func (s *Service) UnassignRole(ctx context.Context, input access.AssignRole) error {
	if err := input.Validate(); err != nil {
		return err
	}
	if err := s.ownRole(ctx, input.RoleID); err != nil {
		return err
	}
	if err := s.ownUser(ctx, input.UserID); err != nil {
		return err
	}
	if err := s.store.UnassignRole(ctx, input, s.organizationID); err != nil {
		return err
	}
	// Issued tokens carry the role's permissions until they expire (up to
	// 15 min); ending the user's sessions makes the removal immediate, like
	// suspension. The unassignment already happened, so a failure here is
	// logged rather than reported: the permission still lapses at expiry.
	if err := s.store.RevokeSessions(ctx, input.UserID); err != nil {
		slog.Warn("role unassigned but revoking the user's sessions failed; "+
			"existing tokens keep the role until they expire",
			"user_id", input.UserID, "role_id", input.RoleID, "error", err)
	}
	return nil
}

func (s *Service) ListAssignments(ctx context.Context) ([]access.RoleAssignment, error) {
	all, err := s.store.ListAssignments(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]access.RoleAssignment, 0, len(all))
	for _, a := range all {
		if a.ResourceID == s.resourceID && a.OrganizationID == s.organizationID {
			out = append(out, a)
		}
	}
	return out, nil
}

var _ access.UserCommands = (*Service)(nil)
var _ access.UserQueries = (*Service)(nil)
var _ access.RoleCommands = (*Service)(nil)
var _ access.RoleQueries = (*Service)(nil)
var _ access.AssignmentCommands = (*Service)(nil)
var _ access.AssignmentQueries = (*Service)(nil)
