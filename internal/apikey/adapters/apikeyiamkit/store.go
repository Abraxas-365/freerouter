package apikeyiamkit

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/iamkit/sdk/iamclient"
)

// Store implements apikey.Store using IAMKit's management API (service accounts).
type Store struct {
	client *iamclient.Client
	envID  string
	env    iamclient.Environment
}

// New creates an IAMKit-backed API key store.
func New(client *iamclient.Client, environmentID string) *Store {
	return &Store{
		client: client,
		envID:  environmentID,
		env:    client.Environment(environmentID),
	}
}

func (s *Store) Create(ctx context.Context, input apikey.CreateAPIKey, applicationID, resourceID string) (apikey.APIKeyCredential, error) {
	cred, err := s.env.CreateServiceAccount(ctx, iamclient.ServiceAccount{
		Name:          input.Name,
		ApplicationID: applicationID,
		ResourceID:    resourceID,
		Permissions:   input.Permissions,
		ExpiresIn:     input.ExpiresIn,
	})
	if err != nil {
		return apikey.APIKeyCredential{}, errx.Wrap(err, "iamkit: create service account", errx.TypeInternal)
	}
	return apikey.APIKeyCredential{
		ID:        cred.ID,
		Secret:    cred.Secret,
		ExpiresAt: cred.ExpiresAt,
	}, nil
}

// paginatedServiceAccounts matches IAMKit's paginated response envelope.
type paginatedServiceAccounts struct {
	Items []iamclient.ServiceAccount `json:"items"`
}

func (s *Store) List(ctx context.Context) ([]apikey.APIKey, error) {
	var page paginatedServiceAccounts
	path := "/environments/" + s.envID + "/service-accounts"
	if err := s.client.Do(ctx, "GET", path, nil, &page); err != nil {
		return nil, errx.Wrap(err, "iamkit: list service accounts", errx.TypeInternal)
	}
	out := make([]apikey.APIKey, len(page.Items))
	for i, a := range page.Items {
		out[i] = apikey.APIKey{
			ID:            a.ID,
			Name:          a.Name,
			ApplicationID: a.ApplicationID,
			ResourceID:    a.ResourceID,
			Permissions:   a.Permissions,
			ExpiresIn:     a.ExpiresIn,
		}
	}
	return out, nil
}

func (s *Store) Revoke(ctx context.Context, id string) error {
	if err := s.env.RevokeServiceAccount(ctx, id); err != nil {
		return errx.Wrap(err, "iamkit: revoke service account", errx.TypeInternal)
	}
	return nil
}

// ListApplications returns the IAMKit applications registered in this
// environment, so callers can pick which service a new service account
// belongs to.
func (s *Store) ListApplications(ctx context.Context) ([]apikey.Application, error) {
	apps, err := s.env.Applications(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "iamkit: list applications", errx.TypeInternal)
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

var _ apikey.Store = (*Store)(nil)
