package server

import "testing"

func TestValidPermissions_ContainsAllConstants(t *testing.T) {
	all := []string{
		PermGatewayInvoke,
		PermGatewayWrite,
		PermMetricsRead,
		PermProvidersRead,
		PermProvidersWrite,
		PermProviderKeysRead,
		PermProviderKeysWrite,
		PermUsageRead,
		PermUsageWrite,
		PermRateLimitsRead,
		PermRateLimitsWrite,
		PermRoutingRead,
		PermRoutingWrite,
		PermGuardrailsRead,
		PermGuardrailsWrite,
		PermWebhooksRead,
		PermWebhooksWrite,
		PermServiceAccountsRead,
		PermServiceAccountsWrite,
		PermUsersRead,
		PermUsersWrite,
		PermRolesRead,
		PermRolesWrite,
	}
	for _, p := range all {
		if !ValidPermissions[p] {
			t.Errorf("permission %q missing from ValidPermissions map", p)
		}
	}
	if len(all) != len(ValidPermissions) {
		t.Errorf("constant count %d != map size %d — add missing entries", len(all), len(ValidPermissions))
	}
}

func TestDefaultPermissions(t *testing.T) {
	if len(DefaultPermissions) != 1 || DefaultPermissions[0] != PermGatewayInvoke {
		t.Errorf("expected default [gateway:invoke], got %v", DefaultPermissions)
	}
}
