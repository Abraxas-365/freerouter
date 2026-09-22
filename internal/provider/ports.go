package provider

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// ── Provider ────────────────────────────────────────────────────────

// Commands defines write operations for providers.
type Commands interface {
	Create(ctx context.Context, input Create) (identity.ProviderID, error)
	Update(ctx context.Context, provider identity.ProviderID, input Update) error
	Delete(ctx context.Context, provider identity.ProviderID) error
}

// Queries defines read operations for providers.
type Queries interface {
	Find(ctx context.Context, provider identity.ProviderID) (Provider, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[Provider], error)
}

// Repository is the persistence contract for providers.
type Repository interface {
	Create(ctx context.Context, provider identity.ProviderID, input Create) error
	Find(ctx context.Context, provider identity.ProviderID) (Provider, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[Provider], error)
	Update(ctx context.Context, provider identity.ProviderID, input Update) error
	Delete(ctx context.Context, provider identity.ProviderID) error
}

// ── Model ───────────────────────────────────────────────────────────

// ModelCommands defines write operations for models.
type ModelCommands interface {
	CreateModel(ctx context.Context, input CreateModel) (identity.ModelID, error)
	UpdateModel(ctx context.Context, model identity.ModelID, input UpdateModel) error
	DeleteModel(ctx context.Context, model identity.ModelID) error
}

// ModelQueries defines read operations for models.
type ModelQueries interface {
	FindModel(ctx context.Context, model identity.ModelID) (Model, error)
	FindModelByName(ctx context.Context, name string) (Model, error)
	ListModels(ctx context.Context, filter ModelFilter, page query.Pagination) (query.Paginated[Model], error)
}

// ModelRepository is the persistence contract for models.
type ModelRepository interface {
	Create(ctx context.Context, model identity.ModelID, input CreateModel) error
	Find(ctx context.Context, model identity.ModelID) (Model, error)
	FindByName(ctx context.Context, name string) (Model, error)
	List(ctx context.Context, filter ModelFilter, page query.Pagination) (query.Paginated[Model], error)
	Update(ctx context.Context, model identity.ModelID, input UpdateModel) error
	Delete(ctx context.Context, model identity.ModelID) error
}

// ── Mapping ─────────────────────────────────────────────────────────

// MappingCommands defines write operations for model-provider mappings.
type MappingCommands interface {
	CreateMapping(ctx context.Context, input CreateMapping) (identity.MappingID, error)
	UpdateMapping(ctx context.Context, mapping identity.MappingID, input UpdateMapping) error
	DeleteMapping(ctx context.Context, mapping identity.MappingID) error
}

// MappingQueries defines read operations for model-provider mappings.
type MappingQueries interface {
	FindMapping(ctx context.Context, mapping identity.MappingID) (ModelProviderMapping, error)
	ListMappings(ctx context.Context, filter MappingFilter, page query.Pagination) (query.Paginated[ModelProviderMapping], error)
	// ListActiveMappingsByModel returns all active mappings for a model (no pagination).
	ListActiveMappingsByModel(ctx context.Context, model identity.ModelID) ([]ModelProviderMapping, error)
}

// MappingRepository is the persistence contract for model-provider mappings.
type MappingRepository interface {
	Create(ctx context.Context, mapping identity.MappingID, input CreateMapping) error
	Find(ctx context.Context, mapping identity.MappingID) (ModelProviderMapping, error)
	List(ctx context.Context, filter MappingFilter, page query.Pagination) (query.Paginated[ModelProviderMapping], error)
	ListActiveByModel(ctx context.Context, model identity.ModelID) ([]ModelProviderMapping, error)
	Update(ctx context.Context, mapping identity.MappingID, input UpdateMapping) error
	Delete(ctx context.Context, mapping identity.MappingID) error
}

// ── Fallback ────────────────────────────────────────────────────────

// FallbackCommands defines write operations for model fallbacks.
type FallbackCommands interface {
	CreateFallback(ctx context.Context, input CreateFallback) (identity.ModelFallbackID, error)
	DeleteFallback(ctx context.Context, fallback identity.ModelFallbackID) error
}

// FallbackQueries defines read operations for model fallbacks.
type FallbackQueries interface {
	ListFallbacks(ctx context.Context, model identity.ModelID) ([]ModelFallback, error)
}

// FallbackRepository is the persistence contract for model fallbacks.
type FallbackRepository interface {
	Create(ctx context.Context, fallback identity.ModelFallbackID, input CreateFallback) error
	FindByModel(ctx context.Context, model identity.ModelID) ([]ModelFallback, error)
	Delete(ctx context.Context, fallback identity.ModelFallbackID) error
}
