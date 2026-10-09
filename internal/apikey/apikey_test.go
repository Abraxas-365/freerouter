package apikey

import (
	"testing"

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
