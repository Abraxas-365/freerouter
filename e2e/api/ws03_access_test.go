//go:build e2e

// WS-03 — Access management (users, roles, role assignments) functional tests.
//
// Everything created here is prefixed ws03- and removed in t.Cleanup. Users
// are removed with IAMKit's permanent delete (management API) because
// FreeRouter's DELETE /access/users/:id only suspends (soft delete).
//
// FreeRouter's backend reaches IAMKit /api/v1 with one service account whose
// budget is IAMKit's per-IP limit (120 req/min) shared by every concurrent
// e2e worker; an IAMKit 429 surfaces as a FreeRouter 502 "… in IAMKit"
// (finding WS03-6). frDo therefore retries those 502s so this suite measures
// behaviour, not the shared budget.
package api

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

const ws03Password = "ws03-password-123456" // satisfies IAMKit's 12-72 policy

// ── helpers ─────────────────────────────────────────────────────────

// retryable reports whether r is an upstream-throttling artefact.
func ws03Retryable(r Resp) bool {
	if r.Status == http.StatusTooManyRequests {
		return true
	}
	return r.Status == http.StatusBadGateway && bytes.Contains(r.Body, []byte("in IAMKit"))
}

func ws03Do(t *testing.T, c *Client, method, path string, body any) Resp {
	t.Helper()
	deadline := time.Now().Add(150 * time.Second)
	for {
		r := c.Do(t, method, path, body)
		if !ws03Retryable(r) || time.Now().After(deadline) {
			return r
		}
		time.Sleep(3 * time.Second)
	}
}

// frDo calls FreeRouter /api/v1 as the admin service account.
func frDo(t *testing.T, method, path string, body any) Resp {
	t.Helper()
	return ws03Do(t, AdminAPI(t), method, path, body)
}

// mgDo calls IAMKit's management API inside the e2e environment.
func mgDo(t *testing.T, method, path string, body any) Resp {
	t.Helper()
	return ws03Do(t, Management(t), method, "/environments/"+FX(t).IAMKit.EnvironmentID+path, body)
}

func ws03Email(tag string) string { return Uniq("ws03-"+tag) + "@e2e.test" }

func wantStatus(t *testing.T, r Resp, want int, what string) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("%s: want %d, got %d %s", what, want, r.Status, r.Body)
	}
}

func wantError(t *testing.T, r Resp, status int, msgContains, what string) {
	t.Helper()
	wantStatus(t, r, status, what)
	var e struct {
		Message string `json:"message"`
	}
	r.JSON(t, &e)
	if !strings.Contains(e.Message, msgContains) {
		t.Fatalf("%s: message %q does not contain %q", what, e.Message, msgContains)
	}
}

type ws03User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type ws03Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type ws03Assignment struct {
	UserID         string `json:"user_id"`
	RoleID         string `json:"role_id"`
	OrganizationID string `json:"organization_id"`
}

// purgeUser permanently deletes a user (cleanup only; the product only suspends).
func purgeUser(t *testing.T, id string) {
	t.Cleanup(func() {
		r := mgDo(t, http.MethodDelete, "/users/"+id+"/permanent", nil)
		if r.Status != 204 && r.Status != 404 {
			t.Logf("cleanup user %s: %d %s", id, r.Status, r.Body)
		}
	})
}

// newUser creates a user through FreeRouter and registers its cleanup.
func newUser(t *testing.T, tag string) ws03User {
	t.Helper()
	in := map[string]string{"email": ws03Email(tag), "name": "WS03 " + tag, "password": ws03Password}
	r := frDo(t, http.MethodPost, "/access/users", in)
	wantStatus(t, r, 201, "create user")
	var u ws03User
	r.JSON(t, &u)
	purgeUser(t, u.ID)
	return u
}

// newForeignUser creates a user homed in the foreign organization (management API).
func newForeignUser(t *testing.T) string {
	t.Helper()
	f := FX(t)
	r := mgDo(t, http.MethodPost, "/users", map[string]string{
		"name": "WS03 Foreign", "email": ws03Email("foreign"), "password": ws03Password,
		"home_organization_id": f.Boundary["foreign_org_id"],
	})
	wantStatus(t, r, 201, "mgmt create foreign user")
	id, _ := r.Map(t)["id"].(string)
	purgeUser(t, id)
	return id
}

// newRole creates a role through FreeRouter and registers its cleanup.
func newRole(t *testing.T, tag string, perms ...string) ws03Role {
	t.Helper()
	r := frDo(t, http.MethodPost, "/access/roles", map[string]any{"name": Uniq("ws03-" + tag), "permissions": perms})
	wantStatus(t, r, 201, "create role")
	var role ws03Role
	r.JSON(t, &role)
	t.Cleanup(func() { frDo(t, http.MethodDelete, "/access/roles/"+role.ID, nil) })
	return role
}

func listUsers(t *testing.T) []ws03User {
	t.Helper()
	r := frDo(t, http.MethodGet, "/access/users", nil)
	wantStatus(t, r, 200, "list users")
	var out []ws03User
	r.JSON(t, &out)
	return out
}

func listRoles(t *testing.T) []ws03Role {
	t.Helper()
	r := frDo(t, http.MethodGet, "/access/roles", nil)
	wantStatus(t, r, 200, "list roles")
	var out []ws03Role
	r.JSON(t, &out)
	return out
}

func listAssignments(t *testing.T) []ws03Assignment {
	t.Helper()
	r := frDo(t, http.MethodGet, "/access/role-assignments", nil)
	wantStatus(t, r, 200, "list assignments")
	var out []ws03Assignment
	r.JSON(t, &out)
	return out
}

func findUser(us []ws03User, id string) (ws03User, bool) {
	for _, u := range us {
		if u.ID == id {
			return u, true
		}
	}
	return ws03User{}, false
}

func hasRole(rs []ws03Role, id string) bool {
	for _, r := range rs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func hasAssignment(as []ws03Assignment, user, role string) bool {
	for _, a := range as {
		if a.UserID == user && a.RoleID == role {
			return true
		}
	}
	return false
}

func assign(t *testing.T, user, role string) Resp {
	t.Helper()
	return frDo(t, http.MethodPost, "/access/role-assignments", map[string]string{"user_id": user, "role_id": role})
}

func unassign(t *testing.T, user, role string) Resp {
	t.Helper()
	return frDo(t, http.MethodDelete, "/access/role-assignments", map[string]string{"user_id": user, "role_id": role})
}

func randomUUID() string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", time.Now().UnixNano()%1_000_000_000_000)
}

// ── users ───────────────────────────────────────────────────────────

func TestWS03_UserCreate(t *testing.T) {
	FX(t)

	t.Run("valid user is created active, listed and findable", func(t *testing.T) {
		email := ws03Email("valid")
		r := frDo(t, http.MethodPost, "/access/users", map[string]string{"email": email, "name": "WS03 Valid", "password": ws03Password})
		wantStatus(t, r, 201, "create")
		var u ws03User
		r.JSON(t, &u)
		purgeUser(t, u.ID)
		if u.ID == "" || u.Email != email || u.Name != "WS03 Valid" || !u.Active {
			t.Fatalf("unexpected body %+v", u)
		}
		if got, ok := findUser(listUsers(t), u.ID); !ok || got.Email != email {
			t.Fatalf("created user not in list (%v %+v)", ok, got)
		}
		var found ws03User
		r = frDo(t, http.MethodGet, "/access/users/"+u.ID, nil)
		wantStatus(t, r, 200, "find")
		r.JSON(t, &found)
		if found != u {
			t.Fatalf("find = %+v, want %+v", found, u)
		}
	})

	t.Run("duplicate email is 409 with a readable message (also case-insensitive)", func(t *testing.T) {
		u := newUser(t, "dup")
		for _, email := range []string{u.Email, strings.ToUpper(u.Email)} {
			r := frDo(t, http.MethodPost, "/access/users", map[string]string{"email": email, "name": "Dup", "password": ws03Password})
			wantError(t, r, 409, "email is taken", "duplicate "+email)
		}
	})

	t.Run("field validation", func(t *testing.T) {
		long := strings.Repeat("a", 73)
		cases := []struct {
			name   string
			body   any
			status int
			msg    string
		}{
			{"invalid email", map[string]string{"email": "not-an-email", "name": "X", "password": ws03Password}, 400, "valid email"},
			{"missing email", map[string]string{"name": "X", "password": ws03Password}, 400, "email is required"},
			{"missing name", map[string]string{"email": ws03Email("noname"), "password": ws03Password}, 400, "name is required"},
			{"blank name", map[string]string{"email": ws03Email("blank"), "name": "   ", "password": ws03Password}, 400, "name is required"},
			{"password 7 chars", map[string]string{"email": ws03Email("pw7"), "name": "X", "password": "1234567"}, 400, "at least 8"},
			{"password 11 chars (IAMKit policy)", map[string]string{"email": ws03Email("pw11"), "name": "X", "password": "12345678901"}, 400, "12-72"},
			{"password 73 chars", map[string]string{"email": ws03Email("pw73"), "name": "X", "password": long}, 400, "at most 72"},
			{"malformed JSON", `{"email":`, 400, "invalid request body"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				r := frDo(t, http.MethodPost, "/access/users", c.body)
				wantError(t, r, c.status, c.msg, c.name)
			})
		}
	})

	t.Run("password boundaries 12 and 72 are accepted", func(t *testing.T) {
		for _, pw := range []string{strings.Repeat("p", 12), strings.Repeat("p", 72)} {
			r := frDo(t, http.MethodPost, "/access/users", map[string]string{"email": ws03Email("pwok"), "name": "PW", "password": pw})
			wantStatus(t, r, 201, fmt.Sprintf("password len %d", len(pw)))
			purgeUser(t, r.Map(t)["id"].(string))
		}
	})

	t.Run("unicode and very long names round-trip", func(t *testing.T) {
		for _, name := range []string{"Zoë Ünïcødé 名前 🚀", strings.Repeat("N", 300)} {
			r := frDo(t, http.MethodPost, "/access/users", map[string]string{"email": ws03Email("uni"), "name": name, "password": ws03Password})
			wantStatus(t, r, 201, "create")
			var u ws03User
			r.JSON(t, &u)
			purgeUser(t, u.ID)
			if u.Name != name {
				t.Fatalf("name %q round-tripped as %q", name, u.Name)
			}
		}
	})

	// FINDING WS03-4: surrounding whitespace is stored verbatim in the email.
	t.Run("email with surrounding whitespace is trimmed or rejected", func(t *testing.T) {
		email := ws03Email("space")
		r := frDo(t, http.MethodPost, "/access/users", map[string]string{"email": "  " + email + "  ", "name": "Space", "password": ws03Password})
		if r.Status == 201 {
			var u ws03User
			r.JSON(t, &u)
			purgeUser(t, u.ID)
			if u.Email != email {
				t.Fatalf("email stored as %q, want trimmed %q", u.Email, email)
			}
			return
		}
		wantStatus(t, r, 400, "padded email")
	})
}

func TestWS03_UserUpdateAndSuspend(t *testing.T) {
	FX(t)

	t.Run("rename is persisted; blank name rejected", func(t *testing.T) {
		u := newUser(t, "rename")
		wantStatus(t, frDo(t, http.MethodPatch, "/access/users/"+u.ID, map[string]string{"name": "WS03 Renamed ✓"}), 204, "rename")
		var got ws03User
		frDo(t, http.MethodGet, "/access/users/"+u.ID, nil).JSON(t, &got)
		if got.Name != "WS03 Renamed ✓" || got.Email != u.Email || !got.Active {
			t.Fatalf("after rename: %+v", got)
		}
		wantError(t, frDo(t, http.MethodPatch, "/access/users/"+u.ID, map[string]string{"name": ""}), 400, "name is required", "blank rename")
	})

	t.Run("unknown and malformed ids", func(t *testing.T) {
		id := randomUUID()
		wantStatus(t, frDo(t, http.MethodGet, "/access/users/"+id, nil), 404, "find unknown")
		wantStatus(t, frDo(t, http.MethodPatch, "/access/users/"+id, map[string]string{"name": "x"}), 404, "patch unknown")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/"+id, nil), 404, "suspend unknown")
		wantStatus(t, frDo(t, http.MethodGet, "/access/users/not-a-uuid", nil), 404, "find malformed")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/not-a-uuid", nil), 404, "suspend malformed")
	})

	t.Run("DELETE suspends (soft): user stays listed as inactive, idempotent, reactivatable", func(t *testing.T) {
		u := newUser(t, "suspend")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/"+u.ID, nil), 204, "suspend")
		got, ok := findUser(listUsers(t), u.ID)
		if !ok || got.Active {
			t.Fatalf("suspended user should be listed inactive: listed=%v %+v", ok, got)
		}
		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/"+u.ID, nil), 204, "suspend twice")
		wantStatus(t, frDo(t, http.MethodPatch, "/access/users/"+u.ID, map[string]bool{"active": true}), 204, "reactivate")
		frDo(t, http.MethodGet, "/access/users/"+u.ID, nil).JSON(t, &got)
		if !got.Active {
			t.Fatalf("reactivate via PATCH active=true did not take: %+v", got)
		}
	})

	t.Run("suspension revokes an already-issued token and blocks login", func(t *testing.T) {
		f := FX(t)
		u := newUser(t, "revoke")
		role := newRole(t, "revoke", "freerouter:roles:read")
		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign")
		tok, _ := Login(t, Persona{Email: u.Email, Password: ws03Password})
		me := Bearer(f.URLs.API, tok)
		wantStatus(t, ws03Do(t, me, http.MethodGet, "/access/roles", nil), 200, "before suspend")

		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/"+u.ID, nil), 204, "suspend")
		Eventually(t, 15*time.Second, func() bool {
			return ws03Do(t, me, http.MethodGet, "/access/roles", nil).Status == 401
		}, "token of suspended user still accepted")

		r := Anon(f.URLs.IAMKit).Post(t, "/identity/v1/login", map[string]string{
			"environment_id": f.IAMKit.EnvironmentID, "organization_id": f.IAMKit.OrganizationID,
			"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
			"email": u.Email, "password": ws03Password,
		})
		if r.Status == 200 {
			t.Fatalf("suspended user could log in: %s", r.Body)
		}
	})

	t.Run("self-suspend (documented behaviour)", func(t *testing.T) {
		f := FX(t)
		u := newUser(t, "self")
		role := newRole(t, "self", "freerouter:users:read", "freerouter:users:write")
		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign")
		tok, _ := Login(t, Persona{Email: u.Email, Password: ws03Password})
		me := Bearer(f.URLs.API, tok)
		r := ws03Do(t, me, http.MethodDelete, "/access/users/"+u.ID, nil)
		t.Logf("self-suspend: %d %s", r.Status, r.Body)
		if r.Status == 204 {
			// Allowed: the caller locks itself out; its token must stop working.
			Eventually(t, 15*time.Second, func() bool {
				return ws03Do(t, me, http.MethodGet, "/access/users", nil).Status == 401
			}, "self-suspended token still accepted")
		}
	})
}

// ── roles ───────────────────────────────────────────────────────────

func TestWS03_Roles(t *testing.T) {
	FX(t)

	t.Run("create valid role; listed with its permissions", func(t *testing.T) {
		role := newRole(t, "valid", "freerouter:metrics:read", "freerouter:usage:read")
		if len(role.Permissions) != 2 {
			t.Fatalf("permissions echoed: %v", role.Permissions)
		}
		for _, r := range listRoles(t) {
			if r.ID == role.ID {
				if r.Name != role.Name || len(r.Permissions) != 2 {
					t.Fatalf("listed as %+v", r)
				}
				return
			}
		}
		t.Fatal("created role not listed")
	})

	t.Run("create validation", func(t *testing.T) {
		cases := []struct {
			name string
			body any
			msg  string
		}{
			{"unknown permission", map[string]any{"name": Uniq("ws03-bad"), "permissions": []string{"freerouter:nope"}}, `unknown permission: "freerouter:nope"`},
			{"permission of another resource", map[string]any{"name": Uniq("ws03-bad"), "permissions": []string{"iam:users:read"}}, `unknown permission: "iam:users:read"`},
			{"mixed valid + foreign permission", map[string]any{"name": Uniq("ws03-bad"), "permissions": []string{"freerouter:metrics:read", "iam:roles:write"}}, `unknown permission: "iam:roles:write"`},
			{"empty permissions", map[string]any{"name": Uniq("ws03-bad"), "permissions": []string{}}, "at least one permission"},
			{"missing permissions", map[string]any{"name": Uniq("ws03-bad")}, "at least one permission"},
			{"missing name", map[string]any{"permissions": []string{"freerouter:metrics:read"}}, "name is required"},
			{"blank name", map[string]any{"name": "  ", "permissions": []string{"freerouter:metrics:read"}}, "name is required"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				wantError(t, frDo(t, http.MethodPost, "/access/roles", c.body), 400, c.msg, c.name)
			})
		}
	})

	t.Run("duplicate name is 409", func(t *testing.T) {
		role := newRole(t, "dupname", "freerouter:metrics:read")
		r := frDo(t, http.MethodPost, "/access/roles", map[string]any{"name": role.Name, "permissions": []string{"freerouter:usage:read"}})
		wantStatus(t, r, 409, "duplicate role name")
	})

	// FINDING WS03-7: the 409 says "conflicting or out-of-bound resource".
	t.Run("duplicate name error message is readable", func(t *testing.T) {
		role := newRole(t, "dupmsg", "freerouter:metrics:read")
		r := frDo(t, http.MethodPost, "/access/roles", map[string]any{"name": role.Name, "permissions": []string{"freerouter:usage:read"}})
		wantStatus(t, r, 409, "duplicate role name")
		msg := strings.ToLower(r.Map(t)["message"].(string))
		if !strings.Contains(msg, "name") && !strings.Contains(msg, "exist") && !strings.Contains(msg, "taken") {
			t.Fatalf("409 message does not tell the admin the name is taken: %q", msg)
		}
	})

	t.Run("duplicate permissions in one request (documented)", func(t *testing.T) {
		r := frDo(t, http.MethodPost, "/access/roles", map[string]any{"name": Uniq("ws03-dupperm"), "permissions": []string{"freerouter:metrics:read", "freerouter:metrics:read"}})
		if r.Status == 201 {
			id := r.Map(t)["id"].(string)
			t.Cleanup(func() { frDo(t, http.MethodDelete, "/access/roles/"+id, nil) })
		}
		if r.Status != 201 && r.Status != 400 {
			t.Fatalf("want 201 (deduped) or 400, got %d %s", r.Status, r.Body)
		}
		t.Logf("duplicate permissions: %d %s", r.Status, r.Body)
	})

	t.Run("update name and permissions", func(t *testing.T) {
		role := newRole(t, "upd", "freerouter:metrics:read")
		newName := Uniq("ws03-upd-renamed")
		wantStatus(t, frDo(t, http.MethodPut, "/access/roles/"+role.ID, map[string]any{
			"name": newName, "permissions": []string{"freerouter:usage:read", "freerouter:webhooks:read"},
		}), 204, "update")
		for _, r := range listRoles(t) {
			if r.ID == role.ID {
				if r.Name != newName || len(r.Permissions) != 2 || !contains(r.Permissions, "freerouter:webhooks:read") || contains(r.Permissions, "freerouter:metrics:read") {
					t.Fatalf("after update: %+v", r)
				}
				goto validation
			}
		}
		t.Fatal("updated role missing from list")
	validation:
		wantError(t, frDo(t, http.MethodPut, "/access/roles/"+role.ID, map[string]any{"name": newName, "permissions": []string{"freerouter:nope"}}), 400, "unknown permission", "update unknown perm")
		wantError(t, frDo(t, http.MethodPut, "/access/roles/"+role.ID, map[string]any{"name": newName, "permissions": []string{}}), 400, "at least one permission", "update empty perms")
		wantError(t, frDo(t, http.MethodPut, "/access/roles/"+role.ID, map[string]any{"name": "", "permissions": []string{"freerouter:usage:read"}}), 400, "name is required", "update blank name")
		other := newRole(t, "upd-other", "freerouter:metrics:read")
		wantStatus(t, frDo(t, http.MethodPut, "/access/roles/"+role.ID, map[string]any{"name": other.Name, "permissions": []string{"freerouter:usage:read"}}), 409, "rename onto existing name")
	})

	t.Run("unknown ids are 404", func(t *testing.T) {
		id := randomUUID()
		wantStatus(t, frDo(t, http.MethodPut, "/access/roles/"+id, map[string]any{"name": "x", "permissions": []string{"freerouter:usage:read"}}), 404, "update unknown")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/roles/"+id, nil), 404, "delete unknown")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/roles/not-a-uuid", nil), 404, "delete malformed")
	})

	t.Run("delete; second delete 404", func(t *testing.T) {
		role := newRole(t, "del", "freerouter:metrics:read")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/roles/"+role.ID, nil), 204, "delete")
		if hasRole(listRoles(t), role.ID) {
			t.Fatal("deleted role still listed")
		}
		wantStatus(t, frDo(t, http.MethodDelete, "/access/roles/"+role.ID, nil), 404, "delete again")
	})

	t.Run("delete role in use cascades its assignments", func(t *testing.T) {
		u := newUser(t, "inuse")
		role := newRole(t, "inuse", "freerouter:metrics:read")
		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/roles/"+role.ID, nil), 204, "delete in-use role")
		if hasAssignment(listAssignments(t), u.ID, role.ID) {
			t.Fatal("assignment of deleted role still listed")
		}
	})
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// ── assignments ─────────────────────────────────────────────────────

func TestWS03_Assignments(t *testing.T) {
	f := FX(t)

	t.Run("assign grants the role's permissions; unassign revokes them", func(t *testing.T) {
		u := newUser(t, "grant")
		role := newRole(t, "grant", "freerouter:roles:read")
		// A user without any role on the resource cannot log in at all
		// (IAMKit resource login → 401), so start from a base role.
		r := Anon(f.URLs.IAMKit).Post(t, "/identity/v1/login", map[string]string{
			"environment_id": f.IAMKit.EnvironmentID, "organization_id": f.IAMKit.OrganizationID,
			"application_id": f.IAMKit.ApplicationID, "resource_id": f.IAMKit.ResourceID,
			"email": u.Email, "password": ws03Password,
		})
		wantStatus(t, r, 401, "login without any role")
		base := newRole(t, "grant-base", "freerouter:metrics:read")
		wantStatus(t, assign(t, u.ID, base.ID), 204, "assign base")
		tok, _ := Login(t, Persona{Email: u.Email, Password: ws03Password})
		me := Bearer(f.URLs.API, tok)
		wantStatus(t, ws03Do(t, me, http.MethodGet, "/access/roles", nil), 403, "before assign")

		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign")
		found := false
		for _, a := range listAssignments(t) {
			if a.UserID == u.ID && a.RoleID == role.ID {
				found = true
				if a.OrganizationID != f.IAMKit.OrganizationID {
					t.Fatalf("assignment in org %s, want %s", a.OrganizationID, f.IAMKit.OrganizationID)
				}
			}
		}
		if !found {
			t.Fatal("assignment not listed")
		}
		// Permissions live in the token claims: a fresh login picks them up.
		tok, _ = Login(t, Persona{Email: u.Email, Password: ws03Password})
		me = Bearer(f.URLs.API, tok)
		wantStatus(t, ws03Do(t, me, http.MethodGet, "/access/roles", nil), 200, "after assign")
		wantStatus(t, ws03Do(t, me, http.MethodGet, "/access/users", nil), 403, "permission not granted by role")

		wantStatus(t, unassign(t, u.ID, role.ID), 204, "unassign")
		if hasAssignment(listAssignments(t), u.ID, role.ID) {
			t.Fatal("assignment still listed after unassign")
		}
		tok, _ = Login(t, Persona{Email: u.Email, Password: ws03Password})
		wantStatus(t, ws03Do(t, Bearer(f.URLs.API, tok), http.MethodGet, "/access/roles", nil), 403, "fresh token after unassign")
	})

	// WS03-3 (fixed): unassigning a role signs the user out, so the existing
	// token is rejected (401) instead of keeping the role until expiry.
	t.Run("unassign revokes permissions of already-issued tokens", func(t *testing.T) {
		u := newUser(t, "revrole")
		base := newRole(t, "revrole-base", "freerouter:metrics:read")
		role := newRole(t, "revrole", "freerouter:roles:read")
		wantStatus(t, assign(t, u.ID, base.ID), 204, "assign base")
		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign")
		tok, _ := Login(t, Persona{Email: u.Email, Password: ws03Password})
		me := Bearer(f.URLs.API, tok)
		wantStatus(t, ws03Do(t, me, http.MethodGet, "/access/roles", nil), 200, "with role")
		wantStatus(t, unassign(t, u.ID, role.ID), 204, "unassign")
		Eventually(t, 15*time.Second, func() bool {
			s := ws03Do(t, me, http.MethodGet, "/access/roles", nil).Status
			return s == 401 || s == 403
		}, "unassigned permission still honoured for the existing token")
	})

	t.Run("assign twice is 409, unassign twice is 404", func(t *testing.T) {
		u := newUser(t, "twice")
		role := newRole(t, "twice", "freerouter:metrics:read")
		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign")
		wantError(t, assign(t, u.ID, role.ID), 409, "already holds this role", "assign twice")
		wantStatus(t, unassign(t, u.ID, role.ID), 204, "unassign")
		wantStatus(t, unassign(t, u.ID, role.ID), 404, "unassign twice")
	})

	t.Run("unassign a never-assigned pair is 404", func(t *testing.T) {
		u := newUser(t, "never")
		role := newRole(t, "never", "freerouter:metrics:read")
		wantStatus(t, unassign(t, u.ID, role.ID), 404, "unassign not assigned")
	})

	t.Run("assign to suspended user is allowed (documented)", func(t *testing.T) {
		u := newUser(t, "susp")
		role := newRole(t, "susp", "freerouter:metrics:read")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/"+u.ID, nil), 204, "suspend")
		wantStatus(t, assign(t, u.ID, role.ID), 204, "assign to suspended")
		if !hasAssignment(listAssignments(t), u.ID, role.ID) {
			t.Fatal("assignment to suspended user not listed")
		}
	})

	t.Run("validation and unknown ids", func(t *testing.T) {
		u := newUser(t, "val")
		role := newRole(t, "val", "freerouter:metrics:read")
		wantError(t, assign(t, "", role.ID), 400, "user_id is required", "missing user")
		wantError(t, assign(t, u.ID, ""), 400, "role_id is required", "missing role")
		wantError(t, frDo(t, http.MethodPost, "/access/role-assignments", `{"user_id":`), 400, "invalid request body", "malformed")
		wantStatus(t, assign(t, u.ID, randomUUID()), 404, "unknown role")
		wantStatus(t, assign(t, u.ID, "not-a-uuid"), 404, "malformed role id")
		wantStatus(t, unassign(t, u.ID, randomUUID()), 404, "unassign unknown role")
		wantStatus(t, unassign(t, randomUUID(), role.ID), 404, "unassign unknown user")
		r := assign(t, randomUUID(), role.ID)
		if r.Status < 400 || r.Status >= 500 {
			t.Fatalf("assign to unknown user: want 4xx, got %d %s", r.Status, r.Body)
		}
		t.Logf("assign to unknown user: %d %s", r.Status, r.Body)
	})
}

// ── security boundary ───────────────────────────────────────────────

func TestWS03_Boundary(t *testing.T) {
	f := FX(t)
	foreignRole := f.Boundary["foreign_role_id"]
	foreignUser := f.Boundary["foreign_user_id"]
	backendSA := f.Boundary["backend_service_account_id"]

	t.Run("foreign IAM-resource role is invisible and untouchable", func(t *testing.T) {
		if hasRole(listRoles(t), foreignRole) {
			t.Fatal("foreign role listed")
		}
		for _, r := range listRoles(t) {
			for _, p := range r.Permissions {
				if !strings.HasPrefix(p, "freerouter:") {
					t.Fatalf("role %s exposes non-freerouter permission %q", r.Name, p)
				}
			}
		}
		u := newUser(t, "bfr")
		wantError(t, frDo(t, http.MethodPut, "/access/roles/"+foreignRole, map[string]any{"name": "ws03-hijack", "permissions": []string{"freerouter:metrics:read"}}), 404, "role not found", "update foreign role")
		wantError(t, frDo(t, http.MethodDelete, "/access/roles/"+foreignRole, nil), 404, "role not found", "delete foreign role")
		wantError(t, assign(t, u.ID, foreignRole), 404, "role not found", "assign foreign role")
		wantError(t, unassign(t, u.ID, foreignRole), 404, "role not found", "unassign foreign role")
		var role struct {
			Name        string   `json:"name"`
			Permissions []string `json:"permissions"`
		}
		mgDo(t, http.MethodGet, "/roles/"+foreignRole, nil).JSON(t, &role)
		if role.Name != "e2e-foreign-iam-role" || len(role.Permissions) != 1 {
			t.Fatalf("foreign role was modified: %+v", role)
		}
	})

	t.Run("assignments list hides other resources and other organizations", func(t *testing.T) {
		// Out-of-boundary assignments made directly in IAMKit:
		// (a) our user + IAM-resource role in our org, (b) foreign-org user + a
		// FreeRouter role in the foreign org.
		u := newUser(t, "bassign")
		fu := newForeignUser(t)
		role := newRole(t, "bassign", "freerouter:metrics:read")
		a := map[string]string{"organization_id": f.IAMKit.OrganizationID, "user_id": u.ID, "role_id": foreignRole}
		b := map[string]string{"organization_id": f.Boundary["foreign_org_id"], "user_id": fu, "role_id": role.ID}
		for _, in := range []map[string]string{a, b} {
			wantStatus(t, mgDo(t, http.MethodPost, "/role-assignments", in), 204, "mgmt assign")
			in := in
			t.Cleanup(func() {
				mgDo(t, http.MethodDelete, "/role-assignments/"+in["role_id"]+"/"+in["organization_id"]+"/"+in["user_id"], nil)
			})
		}
		for _, as := range listAssignments(t) {
			if as.RoleID == foreignRole || as.OrganizationID != f.IAMKit.OrganizationID || as.UserID == fu {
				t.Fatalf("out-of-boundary assignment listed: %+v", as)
			}
		}
	})

	t.Run("backend service account is invisible and untouchable", func(t *testing.T) {
		if _, ok := findUser(listUsers(t), backendSA); ok {
			t.Fatal("backend service account listed as a user")
		}
		wantStatus(t, frDo(t, http.MethodGet, "/access/users/"+backendSA, nil), 404, "find SA")
		wantStatus(t, frDo(t, http.MethodPatch, "/access/users/"+backendSA, map[string]string{"name": "ws03-hijack"}), 404, "patch SA")
		wantStatus(t, frDo(t, http.MethodDelete, "/access/users/"+backendSA, nil), 404, "suspend SA")
		role := newRole(t, "bsa", "freerouter:metrics:read")
		r := assign(t, backendSA, role.ID)
		if r.Status/100 == 2 {
			t.Fatalf("assigned a role to the backend service account: %d", r.Status)
		}
	})

	// FINDING WS03-1 (critical): users of other organizations are listed.
	t.Run("foreign-org user is not listed", func(t *testing.T) {
		if u, ok := findUser(listUsers(t), foreignUser); ok {
			t.Fatalf("foreign-org user exposed in /access/users: %+v", u)
		}
	})

	// FINDING WS03-1
	t.Run("foreign-org user find is 404", func(t *testing.T) {
		r := frDo(t, http.MethodGet, "/access/users/"+foreignUser, nil)
		wantStatus(t, r, 404, "find foreign-org user")
	})

	// FINDING WS03-1: rename + suspend of another organization's user succeed.
	t.Run("foreign-org user cannot be updated or suspended", func(t *testing.T) {
		fu := newForeignUser(t) // our own foreign-org user: never mutate seeded objects
		r1 := frDo(t, http.MethodPatch, "/access/users/"+fu, map[string]string{"name": "ws03 hijacked"})
		r2 := frDo(t, http.MethodDelete, "/access/users/"+fu, nil)
		var after struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}
		mgDo(t, http.MethodGet, "/users/"+fu, nil).JSON(t, &after)
		if r1.Status != 404 || r2.Status != 404 || after.Name != "WS03 Foreign" || !after.Active {
			t.Fatalf("foreign-org user mutated through FreeRouter: PATCH=%d DELETE=%d, now name=%q active=%v",
				r1.Status, r2.Status, after.Name, after.Active)
		}
	})

	t.Run("assign/unassign on foreign-org user is refused without existence oracle", func(t *testing.T) {
		role := newRole(t, "bfu", "freerouter:metrics:read")
		rForeign := assign(t, foreignUser, role.ID)
		rRandom := assign(t, randomUUID(), role.ID)
		if rForeign.Status/100 == 2 {
			t.Fatalf("assigned a FreeRouter role to a foreign-org user")
		}
		if rForeign.Status != rRandom.Status || !bytes.Equal(rForeign.Body, rRandom.Body) {
			t.Fatalf("foreign vs nonexistent user distinguishable: %d %s / %d %s", rForeign.Status, rForeign.Body, rRandom.Status, rRandom.Body)
		}
		wantStatus(t, unassign(t, foreignUser, role.ID), 404, "unassign foreign-org user")
		if hasAssignment(listAssignments(t), foreignUser, role.ID) {
			t.Fatal("foreign-org assignment listed")
		}
	})

	// FINDING WS03-5: assign on an out-of-org user answers 409 (not 404 like the rest of the boundary).
	t.Run("assign on foreign-org user is 404", func(t *testing.T) {
		role := newRole(t, "bfu404", "freerouter:metrics:read")
		wantStatus(t, assign(t, foreignUser, role.ID), 404, "assign to foreign-org user")
	})
}

// ── authorization ───────────────────────────────────────────────────

func TestWS03_Authz(t *testing.T) {
	f := FX(t)
	viewer := AsUser(t, "viewer")
	providers := AsUser(t, "providers_only")
	anon := Anon(f.URLs.API)
	gw := Bearer(f.URLs.API, f.Personas["gw_key"].Secret)
	someUser := f.Personas["noperm"].UserID

	for _, p := range []string{"/access/users", "/access/roles", "/access/role-assignments"} {
		wantStatus(t, ws03Do(t, viewer, http.MethodGet, p, nil), 200, "viewer GET "+p)
		wantStatus(t, ws03Do(t, providers, http.MethodGet, p, nil), 403, "providers_only GET "+p)
		wantStatus(t, ws03Do(t, gw, http.MethodGet, p, nil), 403, "gw_key GET "+p)
		wantStatus(t, ws03Do(t, anon, http.MethodGet, p, nil), 401, "anon GET "+p)
	}
	writes := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/access/users", map[string]string{"email": ws03Email("authz"), "name": "x", "password": ws03Password}},
		{http.MethodPatch, "/access/users/" + someUser, map[string]string{"name": "ws03 hijack"}},
		{http.MethodDelete, "/access/users/" + someUser, nil},
		{http.MethodPost, "/access/roles", map[string]any{"name": Uniq("ws03-authz"), "permissions": []string{"freerouter:metrics:read"}}},
		{http.MethodPut, "/access/roles/" + f.Roles["viewer"], map[string]any{"name": "ws03-hijack", "permissions": []string{"freerouter:metrics:read"}}},
		{http.MethodDelete, "/access/roles/" + f.Roles["viewer"], nil},
		{http.MethodPost, "/access/role-assignments", map[string]string{"user_id": someUser, "role_id": f.Roles["admin"]}},
		{http.MethodDelete, "/access/role-assignments", map[string]string{"user_id": f.Personas["viewer"].UserID, "role_id": f.Roles["viewer"]}},
	}
	for _, w := range writes {
		wantStatus(t, ws03Do(t, viewer, w.method, w.path, w.body), 403, "viewer "+w.method+" "+w.path)
		wantStatus(t, ws03Do(t, anon, w.method, w.path, w.body), 401, "anon "+w.method+" "+w.path)
	}
}

// ── large lists ─────────────────────────────────────────────────────

func TestWS03_LargeLists(t *testing.T) {
	f := FX(t)
	const nUsers, nRoles = 60, 25
	prefix := Uniq("ws03-bulk")
	userIDs := make([]string, 0, nUsers)
	for i := 0; i < nUsers; i++ {
		r := mgDo(t, http.MethodPost, "/users", map[string]string{
			"name": fmt.Sprintf("WS03 Bulk %02d", i), "email": fmt.Sprintf("%s-%02d@e2e.test", prefix, i),
			"password": ws03Password, "home_organization_id": f.IAMKit.OrganizationID,
		})
		wantStatus(t, r, 201, "mgmt create bulk user")
		id := r.Map(t)["id"].(string)
		purgeUser(t, id)
		userIDs = append(userIDs, id)
	}
	roleIDs := make([]string, 0, nRoles)
	for i := 0; i < nRoles; i++ {
		r := mgDo(t, http.MethodPost, "/roles", map[string]any{
			"name": fmt.Sprintf("%s-role-%02d", prefix, i), "resource_id": f.IAMKit.ResourceID,
			"permissions": []string{"freerouter:metrics:read"},
		})
		wantStatus(t, r, 201, "mgmt create bulk role")
		id := r.Map(t)["id"].(string)
		t.Cleanup(func() { mgDo(t, http.MethodDelete, "/roles/"+id, nil) })
		roleIDs = append(roleIDs, id)
	}

	users := listUsers(t)
	for _, id := range userIDs {
		if _, ok := findUser(users, id); !ok {
			t.Fatalf("bulk user %s missing from list of %d (pagination?)", id, len(users))
		}
	}
	if len(users) < nUsers+6 {
		t.Fatalf("list has %d users, want >= %d", len(users), nUsers+6)
	}
	roles := listRoles(t)
	for _, id := range roleIDs {
		if !hasRole(roles, id) {
			t.Fatalf("bulk role %s missing from list of %d", id, len(roles))
		}
	}
	seen := map[string]bool{}
	for _, u := range users {
		if seen[u.ID] {
			t.Fatalf("user %s listed twice", u.ID)
		}
		seen[u.ID] = true
	}
}
