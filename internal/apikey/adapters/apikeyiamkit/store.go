// Package apikeyiamkit implements apikey.Store with IAMKit service accounts,
// through the permission-scoped API (/api/v1) as FreeRouter's backend
// service account (iam:service-accounts:*, iam:apps:read).
package apikeyiamkit

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/iamx"
	"github.com/Abraxas-365/iamkit/sdk/apiclient"
)

// Store implements apikey.Store using IAMKit service accounts.
type Store struct {
	iam *iamx.Client
}

// New creates an IAMKit-backed API key store.
func New(iam *iamx.Client) *Store {
	return &Store{iam: iam}
}

func (s *Store) Create(ctx context.Context, input apikey.CreateAPIKey, applicationID, resourceID string) (apikey.APIKeyCredential, error) {
	var cred apiclient.ServiceAccountKey
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		cred, err = env.CreateServiceAccount(ctx, apiclient.ServiceAccount{
			Name:          input.Name,
			ApplicationID: applicationID,
			ResourceID:    resourceID,
			Permissions:   input.Permissions,
			ExpiresIn:     input.ExpiresIn,
		})
		return err
	})
	if err != nil {
		return apikey.APIKeyCredential{}, iamx.Translate(err, "create service account")
	}
	return apikey.APIKeyCredential{
		ID:        cred.ID,
		Secret:    cred.Secret,
		ExpiresAt: cred.ExpiresAt,
	}, nil
}

// List returns the environment's active (not revoked) service accounts.
func (s *Store) List(ctx context.Context) ([]apikey.APIKey, error) {
	var accounts []apiclient.ServiceAccount
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		accounts, err = env.ServiceAccounts(ctx)
		return err
	})
	if err != nil {
		return nil, iamx.Translate(err, "list service accounts")
	}
	out := make([]apikey.APIKey, 0, len(accounts))
	for _, a := range accounts {
		if a.RevokedAt != nil {
			continue
		}
		out = append(out, toAPIKey(a))
	}
	return out, nil
}

func (s *Store) Find(ctx context.Context, id string) (apikey.APIKey, error) {
	var a apiclient.ServiceAccount
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		a, err = env.ServiceAccount(ctx, id)
		return err
	})
	if err != nil {
		return apikey.APIKey{}, iamx.Translate(err, "find service account")
	}
	return toAPIKey(a), nil
}

func (s *Store) Revoke(ctx context.Context, id string) error {
	err := s.iam.Do(ctx, func(env apiclient.Environment) error {
		return env.RevokeServiceAccount(ctx, id)
	})
	return iamx.Translate(err, "revoke service account")
}

// ListApplications returns the IAMKit applications registered in this
// environment, so callers can pick which service a new service account
// belongs to.
func (s *Store) ListApplications(ctx context.Context) ([]apikey.Application, error) {
	var apps []apiclient.Application
	err := s.iam.Do(ctx, func(env apiclient.Environment) (err error) {
		apps, err = env.Applications(ctx)
		return err
	})
	if err != nil {
		return nil, iamx.Translate(err, "list applications")
	}
	out := make([]apikey.Application, len(apps))
	for i, a := range apps {
		out[i] = apikey.Application{
			ID:     a.ID,
			Name:   a.Name,
			Active: a.Active,
		}
	}
	return out, nil
}

func toAPIKey(a apiclient.ServiceAccount) apikey.APIKey {
	return apikey.APIKey{
		ID:            a.ID,
		Name:          a.Name,
		ApplicationID: a.ApplicationID,
		ResourceID:    a.ResourceID,
		Permissions:   a.Permissions,
		ExpiresIn:     a.ExpiresIn,
	}
}

var _ apikey.Store = (*Store)(nil)
