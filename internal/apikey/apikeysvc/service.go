package apikeysvc

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/apikey"
)

// Service implements apikey.Commands and apikey.Queries.
// It validates input and delegates to the IAMKit-backed store,
// injecting the FreeRouter application/resource IDs from config.
type Service struct {
	store         apikey.Store
	applicationID string
	resourceID    string
}

// New creates a new API key service.
func New(store apikey.Store, applicationID, resourceID string) *Service {
	return &Service{store: store, applicationID: applicationID, resourceID: resourceID}
}

func (s *Service) Create(ctx context.Context, input apikey.CreateServiceAccount) (apikey.ServiceAccountCredential, error) {
	if err := input.Validate(); err != nil {
		return apikey.ServiceAccountCredential{}, err
	}
	applicationID := input.ApplicationID
	if applicationID == "" {
		applicationID = s.applicationID
	}
	return s.store.Create(ctx, input, applicationID, s.resourceID)
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return s.store.Revoke(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]apikey.APIKey, error) {
	return s.store.List(ctx)
}

func (s *Service) ListApplications(ctx context.Context) ([]apikey.Application, error) {
	return s.store.ListApplications(ctx)
}

var _ apikey.Commands = (*Service)(nil)
var _ apikey.Queries = (*Service)(nil)
