package provider

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ProviderRepository defines the contract for provider persistence
type ProviderRepository interface {
	FindByID(ctx context.Context, id kernel.ProviderID) (*Provider, error)
	FindAll(ctx context.Context) ([]*Provider, error)
	FindActive(ctx context.Context) ([]*Provider, error)
	Save(ctx context.Context, p Provider) error
	Delete(ctx context.Context, id kernel.ProviderID) error
}

// ModelRepository defines the contract for model persistence
type ModelRepository interface {
	FindByID(ctx context.Context, id kernel.ModelID) (*Model, error)
	FindAll(ctx context.Context) ([]*Model, error)
	FindActive(ctx context.Context) ([]*Model, error)
	FindByFamily(ctx context.Context, family string) ([]*Model, error)
	Save(ctx context.Context, m Model) error
	Delete(ctx context.Context, id kernel.ModelID) error
}

// MappingRepository defines the contract for model-provider mapping persistence
type MappingRepository interface {
	FindByID(ctx context.Context, id kernel.MappingID) (*ModelProviderMapping, error)
	FindByModel(ctx context.Context, modelID kernel.ModelID) ([]*ModelProviderMapping, error)
	FindByProvider(ctx context.Context, providerID kernel.ProviderID) ([]*ModelProviderMapping, error)
	FindActiveByModel(ctx context.Context, modelID kernel.ModelID) ([]*ModelProviderMapping, error)
	Save(ctx context.Context, m ModelProviderMapping) error
	Delete(ctx context.Context, id kernel.MappingID) error
}
