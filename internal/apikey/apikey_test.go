package apikey

import (
	"testing"

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
