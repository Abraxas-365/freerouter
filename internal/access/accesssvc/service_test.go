package accesssvc

import (
	"context"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/access"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
)

const (
	ownResource   = "res-freerouter"
	otherResource = "res-iam"
	org           = "org-1"
)

// fakeStore holds two roles: "own" on FreeRouter's resource and "admin" on
// another resource (e.g. the IAM resource), and records mutating calls.
type fakeStore struct {
	access.Store
	calls       []string
	createdOrg  string
	assignments []access.RoleAssignment
	revokeErr   error
}

func (f *fakeStore) FindRole(_ context.Context, id string) (access.Role, error) {
	switch id {
	case "own":
		return access.Role{ID: id, ResourceID: ownResource}, nil
	case "admin":
		return access.Role{ID: id, ResourceID: otherResource}, nil
	}
	return access.Role{}, errx.NotFound("role not found")
}

func (f *fakeStore) ListRoles(context.Context) ([]access.Role, error) {
	return []access.Role{{ID: "own", ResourceID: ownResource}, {ID: "admin", ResourceID: otherResource}}, nil
}

func (f *fakeStore) AssignRole(_ context.Context, in access.AssignRole, _ string) error {
	f.calls = append(f.calls, "assign:"+in.RoleID)
	return nil
}

func (f *fakeStore) UnassignRole(_ context.Context, in access.AssignRole, _ string) error {
	f.calls = append(f.calls, "unassign:"+in.RoleID)
	return nil
}

func (f *fakeStore) UpdateRole(_ context.Context, id string, _ access.UpdateRole, _ string) error {
	f.calls = append(f.calls, "update:"+id)
	return nil
}

func (f *fakeStore) DeleteRole(_ context.Context, id string) error {
	f.calls = append(f.calls, "delete:"+id)
	return nil
}

func (f *fakeStore) CreateUser(_ context.Context, in access.CreateUser, organizationID string) (access.User, error) {
	f.createdOrg = organizationID
	return access.User{ID: "u1", Email: in.Email}, nil
}

func (f *fakeStore) ListAssignments(context.Context) ([]access.RoleAssignment, error) {
	return f.assignments, nil
}

// Users: "u" is homed in FreeRouter's organization, "foreign" in another
// one and "nohome" in none.
var fakeUsers = map[string]access.User{
	"u":       {ID: "u", HomeOrganizationID: org},
	"foreign": {ID: "foreign", HomeOrganizationID: "other-org"},
	"nohome":  {ID: "nohome"},
}

func (f *fakeStore) ListUsers(context.Context) ([]access.User, error) {
	return []access.User{fakeUsers["u"], fakeUsers["foreign"], fakeUsers["nohome"]}, nil
}

func (f *fakeStore) FindUser(_ context.Context, id string) (access.User, error) {
	if u, ok := fakeUsers[id]; ok {
		return u, nil
	}
	return access.User{}, errx.NotFound("user not found")
}

func (f *fakeStore) UpdateUser(_ context.Context, id string, _ access.UpdateUser) error {
	f.calls = append(f.calls, "update-user:"+id)
	return nil
}

func (f *fakeStore) SuspendUser(_ context.Context, id string) error {
	f.calls = append(f.calls, "suspend-user:"+id)
	return nil
}

func (f *fakeStore) RevokeSessions(_ context.Context, id string) error {
	f.calls = append(f.calls, "revoke-sessions:"+id)
	return f.revokeErr
}

func TestUnassignRevokesSessions(t *testing.T) {
	ctx := context.Background()
	in := access.AssignRole{UserID: "u", RoleID: "own"}

	store := &fakeStore{}
	if err := New(store, ownResource, org).UnassignRole(ctx, in); err != nil {
		t.Fatal(err)
	}
	want := []string{"unassign:own", "revoke-sessions:u"}
	if len(store.calls) != 2 || store.calls[0] != want[0] || store.calls[1] != want[1] {
		t.Fatalf("calls = %v, want %v (unassign, then sign the user out)", store.calls, want)
	}

	// The unassignment already happened: a failed sign-out is logged, not
	// reported, so the admin isn't told a successful removal failed.
	failing := &fakeStore{revokeErr: errx.External("iamkit down")}
	if err := New(failing, ownResource, org).UnassignRole(ctx, in); err != nil {
		t.Fatalf("UnassignRole with failing revoke = %v, want nil", err)
	}

	// Assigning a role does not sign the user out.
	assign := &fakeStore{}
	if err := New(assign, ownResource, org).AssignRole(ctx, in); err != nil {
		t.Fatal(err)
	}
	if len(assign.calls) != 1 || assign.calls[0] != "assign:own" {
		t.Fatalf("assign calls = %v, want only assign", assign.calls)
	}
}

func TestForeignRolesAreInvisible(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := New(store, ownResource, org)
	update := access.UpdateRole{Name: "x", Permissions: []string{server.PermGatewayInvoke}}

	checks := map[string]error{
		"assign":   svc.AssignRole(ctx, access.AssignRole{UserID: "u", RoleID: "admin"}),
		"unassign": svc.UnassignRole(ctx, access.AssignRole{UserID: "u", RoleID: "admin"}),
		"update":   svc.UpdateRole(ctx, "admin", update),
		"delete":   svc.DeleteRole(ctx, "admin"),
	}
	for op, err := range checks {
		if !isNotFound(err) {
			t.Errorf("%s foreign role: err = %v, want NOT_FOUND", op, err)
		}
	}
	if len(store.calls) != 0 {
		t.Fatalf("store mutated for a foreign role: %v", store.calls)
	}

	roles, err := svc.ListRoles(ctx)
	if err != nil || len(roles) != 1 || roles[0].ID != "own" {
		t.Fatalf("ListRoles = %v, %v; want only the own role", roles, err)
	}
}

func TestOwnRolesAreManaged(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := New(store, ownResource, org)

	if err := svc.AssignRole(ctx, access.AssignRole{UserID: "u", RoleID: "own"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRole(ctx, "own"); err != nil {
		t.Fatal(err)
	}
	if len(store.calls) != 2 {
		t.Fatalf("calls = %v, want assign and delete", store.calls)
	}
}

func TestCreateUserJoinsConfiguredOrganization(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, ownResource, org)
	if _, err := svc.CreateUser(context.Background(), access.CreateUser{Email: "a@b.co", Name: "A", Password: "longenough"}); err != nil {
		t.Fatal(err)
	}
	if store.createdOrg != org {
		t.Fatalf("home organization = %q, want %q", store.createdOrg, org)
	}
}

func TestListAssignmentsOnlyOwn(t *testing.T) {
	store := &fakeStore{assignments: []access.RoleAssignment{
		{RoleID: "own", OrganizationID: org, ResourceID: ownResource},
		{RoleID: "admin", OrganizationID: org, ResourceID: otherResource},
		{RoleID: "own", OrganizationID: "other-org", ResourceID: ownResource},
	}}
	got, err := New(store, ownResource, org).ListAssignments(context.Background())
	if err != nil || len(got) != 1 || got[0].RoleID != "own" || got[0].OrganizationID != org {
		t.Fatalf("ListAssignments = %v, %v; want only the own role in the configured org", got, err)
	}
}

func TestForeignUsersAreInvisible(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := New(store, ownResource, org)

	users, err := svc.ListUsers(ctx)
	if err != nil || len(users) != 1 || users[0].ID != "u" {
		t.Fatalf("ListUsers = %v, %v; want only the own-org user", users, err)
	}

	name := "x"
	for _, id := range []string{"foreign", "nohome", "missing"} {
		_, findErr := svc.FindUser(ctx, id)
		checks := map[string]error{
			"find":     findErr,
			"update":   svc.UpdateUser(ctx, id, access.UpdateUser{Name: &name}),
			"suspend":  svc.SuspendUser(ctx, id),
			"assign":   svc.AssignRole(ctx, access.AssignRole{UserID: id, RoleID: "own"}),
			"unassign": svc.UnassignRole(ctx, access.AssignRole{UserID: id, RoleID: "own"}),
		}
		for op, err := range checks {
			if !isNotFound(err) {
				t.Errorf("%s %s user: err = %v, want NOT_FOUND", op, id, err)
			}
		}
	}
	if len(store.calls) != 0 {
		t.Fatalf("store mutated for a foreign user: %v", store.calls)
	}

	if err := svc.UpdateUser(ctx, "u", access.UpdateUser{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SuspendUser(ctx, "u"); err != nil {
		t.Fatal(err)
	}
	if len(store.calls) != 2 {
		t.Fatalf("calls = %v, want update and suspend of the own user", store.calls)
	}
}
