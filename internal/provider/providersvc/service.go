package providersvc

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// Service implements all provider module Commands and Queries interfaces.
type Service struct {
	providers provider.Repository
	models    provider.ModelRepository
	mappings  provider.MappingRepository
	fallbacks provider.FallbackRepository
}

var _ provider.Commands = (*Service)(nil)
var _ provider.Queries = (*Service)(nil)
var _ provider.ModelCommands = (*Service)(nil)
var _ provider.ModelQueries = (*Service)(nil)
var _ provider.MappingCommands = (*Service)(nil)
var _ provider.MappingQueries = (*Service)(nil)
var _ provider.FallbackCommands = (*Service)(nil)
var _ provider.FallbackQueries = (*Service)(nil)

// New creates a provider service.
func New(
	providers provider.Repository,
	models provider.ModelRepository,
	mappings provider.MappingRepository,
	fallbacks provider.FallbackRepository,
) *Service {
	return &Service{
		providers: providers,
		models:    models,
		mappings:  mappings,
		fallbacks: fallbacks,
	}
}

// ── Provider ────────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, input provider.Create) (identity.ProviderID, error) {
	if err := input.Validate(); err != nil {
		return identity.ProviderID{}, err
	}
	id := identity.NewProviderID()
	if err := s.providers.Create(ctx, id, input); err != nil {
		return identity.ProviderID{}, err
	}
	return id, nil
}

func (s *Service) Find(ctx context.Context, id identity.ProviderID) (provider.Provider, error) {
	return s.providers.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, filter provider.Filter, page query.Pagination) (query.Paginated[provider.Provider], error) {
	return s.providers.List(ctx, filter, page.Normalize())
}

func (s *Service) Update(ctx context.Context, id identity.ProviderID, input provider.Update) error {
	if err := input.Validate(); err != nil {
		return err
	}
	return s.providers.Update(ctx, id, input)
}

func (s *Service) Delete(ctx context.Context, id identity.ProviderID) error {
	return s.providers.Delete(ctx, id)
}

// ── Model ───────────────────────────────────────────────────────────

func (s *Service) CreateModel(ctx context.Context, input provider.CreateModel) (identity.ModelID, error) {
	if err := input.Validate(); err != nil {
		return identity.ModelID{}, err
	}
	id := identity.NewModelID()
	if err := s.models.Create(ctx, id, input); err != nil {
		return identity.ModelID{}, err
	}
	return id, nil
}

func (s *Service) FindModel(ctx context.Context, id identity.ModelID) (provider.Model, error) {
	return s.models.Find(ctx, id)
}

func (s *Service) FindModelByName(ctx context.Context, name string) (provider.Model, error) {
	return s.models.FindByName(ctx, name)
}

func (s *Service) ListModels(ctx context.Context, filter provider.ModelFilter, page query.Pagination) (query.Paginated[provider.Model], error) {
	return s.models.List(ctx, filter, page.Normalize())
}

func (s *Service) UpdateModel(ctx context.Context, id identity.ModelID, input provider.UpdateModel) error {
	if err := input.Validate(); err != nil {
		return err
	}
	return s.models.Update(ctx, id, input)
}

func (s *Service) DeleteModel(ctx context.Context, id identity.ModelID) error {
	return s.models.Delete(ctx, id)
}

// ── Mapping ─────────────────────────────────────────────────────────

func (s *Service) CreateMapping(ctx context.Context, input provider.CreateMapping) (identity.MappingID, error) {
	if err := input.Validate(); err != nil {
		return identity.MappingID{}, err
	}
	id := identity.NewMappingID()
	if err := s.mappings.Create(ctx, id, input); err != nil {
		return identity.MappingID{}, err
	}
	return id, nil
}

func (s *Service) FindMapping(ctx context.Context, id identity.MappingID) (provider.ModelProviderMapping, error) {
	return s.mappings.Find(ctx, id)
}

func (s *Service) ListMappings(ctx context.Context, filter provider.MappingFilter, page query.Pagination) (query.Paginated[provider.ModelProviderMapping], error) {
	return s.mappings.List(ctx, filter, page.Normalize())
}

func (s *Service) ListActiveMappingsByModel(ctx context.Context, model identity.ModelID) ([]provider.ModelProviderMapping, error) {
	return s.mappings.ListActiveByModel(ctx, model)
}

func (s *Service) UpdateMapping(ctx context.Context, id identity.MappingID, input provider.UpdateMapping) error {
	if err := input.Validate(); err != nil {
		return err
	}
	return s.mappings.Update(ctx, id, input)
}

func (s *Service) DeleteMapping(ctx context.Context, id identity.MappingID) error {
	return s.mappings.Delete(ctx, id)
}

// ── Fallback ────────────────────────────────────────────────────────

func (s *Service) CreateFallback(ctx context.Context, input provider.CreateFallback) (identity.ModelFallbackID, error) {
	if err := input.Validate(); err != nil {
		return identity.ModelFallbackID{}, err
	}
	id := identity.NewModelFallbackID()
	if err := s.fallbacks.Create(ctx, id, input); err != nil {
		return identity.ModelFallbackID{}, err
	}
	return id, nil
}

func (s *Service) ListFallbacks(ctx context.Context, model identity.ModelID) ([]provider.ModelFallback, error) {
	return s.fallbacks.FindByModel(ctx, model)
}

func (s *Service) DeleteFallback(ctx context.Context, id identity.ModelFallbackID) error {
	return s.fallbacks.Delete(ctx, id)
}
