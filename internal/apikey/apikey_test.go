package apikey

import (
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/server"
)

func TestCreateServiceAccount_Validate_NameRequired(t *testing.T) {
	cmd := CreateServiceAccount{}
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestCreateServiceAccount_Validate_DefaultsToGatewayInvoke(t *testing.T) {
	cmd := CreateServiceAccount{Name: "my-app"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Permissions) != 1 || cmd.Permissions[0] != server.PermGatewayInvoke {
		t.Fatalf("expected default [gateway:invoke], got %v", cmd.Permissions)
	}
}

func TestCreateServiceAccount_Validate_AcceptsKnownPermissions(t *testing.T) {
	cmd := CreateServiceAccount{
		Name:        "admin-bot",
		Permissions: []string{server.PermGatewayInvoke, server.PermMetricsRead, server.PermUsageRead},
	}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateServiceAccount_Validate_RejectsUnknownPermission(t *testing.T) {
	cmd := CreateServiceAccount{
		Name:        "bad",
		Permissions: []string{server.PermGatewayInvoke, "admin:nuke"},
	}
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected validation error for unknown permission")
	}
}

func TestCreateServiceAccount_AuthorizeGrant(t *testing.T) {
	keyAdmin := []string{server.PermServiceAccountsWrite, server.PermGatewayInvoke}
	cases := []struct {
		name   string
		caller []string
		grant  []string
		ok     bool
	}{
		{"subset of own permissions", keyAdmin, []string{server.PermGatewayInvoke}, true},
		{"exactly own permissions", keyAdmin, keyAdmin, true},
		{"escalation to users:write", keyAdmin, []string{server.PermUsersWrite}, false},
		{"one extra permission", keyAdmin, []string{server.PermGatewayInvoke, server.PermProvidersWrite}, false},
		{"caller without permissions", nil, []string{server.PermGatewayInvoke}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := CreateServiceAccount{Name: "k", Permissions: tc.grant, CallerPermissions: tc.caller}
			err := cmd.AuthorizeGrant()
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok {
				var x *errx.Error
				if !errx.As(err, &x) || x.Type != errx.TypeForbidden {
					t.Fatalf("err = %v, want FORBIDDEN", err)
				}
			}
		})
	}
}

func TestCreateServiceAccount_KeyCannotOutliveCaller(t *testing.T) {
	perms := []string{server.PermServiceAccountsWrite, server.PermGatewayInvoke}
	in2h := time.Now().Add(2 * time.Hour)
	cases := []struct {
		name      string
		callerExp *time.Time
		expiresIn string
		ok        bool
	}{
		{"no caller expiry, never", nil, NeverExpires, true},
		{"shorter than caller", &in2h, "1h", true},
		{"longer than caller", &in2h, "3h", false},
		{"default 24h outlives a 2h caller", &in2h, "", false},
		{"never from an expiring caller", &in2h, NeverExpires, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := CreateServiceAccount{Name: "k", Permissions: []string{server.PermGatewayInvoke},
				ExpiresIn: tc.expiresIn, CallerPermissions: perms, CallerExpiresAt: tc.callerExp}
			err := cmd.AuthorizeGrant()
			if tc.ok != (err == nil) {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestValidate_DefaultPermissionsAreACopy(t *testing.T) {
	cmd := CreateServiceAccount{Name: "k"}
	if err := cmd.Validate(); err != nil {
		t.Fatal(err)
	}
	cmd.Permissions[0] = "mutated"
	if server.DefaultPermissions[0] == "mutated" {
		t.Fatal("Validate shares server.DefaultPermissions")
	}
}
