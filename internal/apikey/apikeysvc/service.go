package apikeysvc

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/apikey"
	"github.com/Abraxas-365/freerouter/internal/errx"
)

// Service implements apikey.Commands and apikey.Queries.
// It validates input and delegates to the IAMKit-backed store,
// injecting the FreeRouter application/resource IDs from config.
//
// FreeRouter API keys are exactly the service accounts on FreeRouter's
// resource. Other service accounts of the environment (FreeRouter's own
// backend credential on the IAM resource, other services' accounts) are
// neither listed nor revocable here.
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
	if err := input.AuthorizeGrant(); err != nil {
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
		return errx.Validation("service account id is required")
	}
	account, err := s.store.Find(ctx, id)
	if err != nil {
		return err
	}
	if account.ResourceID != s.resourceID {
		return errx.NotFound("service account not found")
	}
	return s.store.Revoke(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]apikey.APIKey, error) {
	all, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]apikey.APIKey, 0, len(all))
	for _, a := range all {
		if a.ResourceID == s.resourceID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *Service) ListApplications(ctx context.Context) ([]apikey.Application, error) {
	return s.store.ListApplications(ctx)
}

var _ apikey.Commands = (*Service)(nil)
var _ apikey.Queries = (*Service)(nil)
