package apikeysrv

import (
	"context"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/iam/apikey"
	"github.com/Abraxas-365/freerouter/internal/iam/tenant"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

type keyRepo struct {
	apikey.APIKeyRepository
	key *apikey.APIKey
}

func (r keyRepo) FindByHash(context.Context, string) (*apikey.APIKey, error) { return r.key, nil }
func (r keyRepo) UpdateLastUsed(context.Context, string) error               { return nil }

type tenantRepo struct {
	tenant.TenantRepository
	entity *tenant.Tenant
}

func (r tenantRepo) FindByID(context.Context, kernel.TenantID) (*tenant.Tenant, error) {
	return r.entity, nil
}

func TestDelegationCoverage(t *testing.T) {
	s := &APIKeyService{}
	for _, tc := range []struct {
		grant, caller []string
		allowed       bool
	}{
		{[]string{"gateway:chat"}, []string{"gateway:*"}, true},
		{[]string{"*"}, []string{"gateway:*"}, false},
		{[]string{"gateway:*"}, []string{"gateway:chat", "gateway:read", "gateway:write"}, false},
		{[]string{"platform:admin"}, []string{"*"}, false},
		{[]string{"users:write"}, []string{"*"}, true},
	} {
		if err := s.validateScopes(tc.grant, tc.caller); (err == nil) != tc.allowed {
			t.Errorf("grant %v from %v: %v", tc.grant, tc.caller, err)
		}
	}
}

func TestTenantSuspensionRejectsKeyWithoutCreatorIdentity(t *testing.T) {
	generated, err := apikey.GenerateAPIKey(apikey.KeyPrefixLive)
	if err != nil {
		t.Fatal(err)
	}
	key := &apikey.APIKey{ID: "key", TenantID: "tenant", IsActive: true, Scopes: []string{"gateway:chat"}}
	org := &tenant.Tenant{ID: "tenant", Status: tenant.TenantStatusSuspended}
	s := NewAPIKeyService(keyRepo{key: key}, tenantRepo{entity: org}, nil)
	if _, err := s.ValidateAPIKey(context.Background(), generated.Key); err == nil {
		t.Fatal("suspended tenant key accepted")
	}
	org.Status = tenant.TenantStatusActive
	if _, err := s.ValidateAPIKey(context.Background(), generated.Key); err != nil {
		t.Fatal(err)
	}
}

func TestLifecycleAndShapeValidationBeforePersistence(t *testing.T) {
	s := NewAPIKeyService(nil, nil, nil)
	active := true
	if _, err := s.UpdateAPIKey(context.Background(), "key", "tenant", []string{"*"}, apikey.UpdateAPIKeyRequest{IsActive: &active}); err == nil {
		t.Fatal("key reactivation accepted")
	}
	if _, err := s.CreateAPIKey(context.Background(), "tenant", "user", []string{"*"}, apikey.CreateAPIKeyRequest{}); err == nil {
		t.Fatal("invalid key request accepted")
	}
}
