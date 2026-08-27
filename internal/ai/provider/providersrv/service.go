package providersrv

import (
	"context"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/google/uuid"
)

// ProviderService provides business operations for the provider registry
type ProviderService struct {
	providerRepo provider.ProviderRepository
	modelRepo    provider.ModelRepository
	mappingRepo  provider.MappingRepository
}

// NewProviderService creates a new instance of the provider service
func NewProviderService(
	providerRepo provider.ProviderRepository,
	modelRepo provider.ModelRepository,
	mappingRepo provider.MappingRepository,
) *ProviderService {
	return &ProviderService{
		providerRepo: providerRepo,
		modelRepo:    modelRepo,
		mappingRepo:  mappingRepo,
	}
}

// ============================================================================
// Provider operations
// ============================================================================

func (s *ProviderService) CreateProvider(ctx context.Context, req provider.CreateProviderRequest) (*provider.Provider, error) {
	p := &provider.Provider{
		ID:          kernel.NewProviderID(uuid.NewString()),
		Name:        req.Name,
		Description: req.Description,
		Website:     req.Website,
		Status:      provider.ProviderStatusActive,
		Streaming:   req.Streaming,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.providerRepo.Save(ctx, *p); err != nil {
		return nil, errx.Wrap(err, "failed to save provider", errx.TypeInternal)
	}
	return p, nil
}

func (s *ProviderService) GetProvider(ctx context.Context, id kernel.ProviderID) (*provider.Provider, error) {
	return s.providerRepo.FindByID(ctx, id)
}

func (s *ProviderService) ListProviders(ctx context.Context) (*provider.ProviderListResponse, error) {
	providers, err := s.providerRepo.FindAll(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list providers", errx.TypeInternal)
	}

	dtos := make([]provider.ProviderDTO, len(providers))
	for i, p := range providers {
		dtos[i] = p.ToDTO()
	}
	return &provider.ProviderListResponse{Providers: dtos, Total: len(dtos)}, nil
}

func (s *ProviderService) ListActiveProviders(ctx context.Context) (*provider.ProviderListResponse, error) {
	providers, err := s.providerRepo.FindActive(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list active providers", errx.TypeInternal)
	}

	dtos := make([]provider.ProviderDTO, len(providers))
	for i, p := range providers {
		dtos[i] = p.ToDTO()
	}
	return &provider.ProviderListResponse{Providers: dtos, Total: len(dtos)}, nil
}

func (s *ProviderService) UpdateProvider(ctx context.Context, id kernel.ProviderID, req provider.UpdateProviderRequest) (*provider.Provider, error) {
	p, err := s.providerRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Website != nil {
		p.Website = *req.Website
	}
	if req.Status != nil {
		p.Status = *req.Status
	}
	if req.Streaming != nil {
		p.Streaming = *req.Streaming
	}
	p.UpdatedAt = time.Now().UTC()

	if err := s.providerRepo.Save(ctx, *p); err != nil {
		return nil, errx.Wrap(err, "failed to update provider", errx.TypeInternal)
	}
	return p, nil
}

func (s *ProviderService) DeleteProvider(ctx context.Context, id kernel.ProviderID) error {
	return s.providerRepo.Delete(ctx, id)
}

// ============================================================================
// Model operations
// ============================================================================

func (s *ProviderService) CreateModel(ctx context.Context, req provider.CreateModelRequest) (*provider.Model, error) {
	m := &provider.Model{
		ID:          kernel.NewModelID(uuid.NewString()),
		Name:        req.Name,
		Description: req.Description,
		Family:      req.Family,
		Stability:   provider.StabilityStable,
		Status:      provider.ModelStatusActive,
		Free:        req.Free,
		ReleasedAt:  time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.modelRepo.Save(ctx, *m); err != nil {
		return nil, errx.Wrap(err, "failed to save model", errx.TypeInternal)
	}
	return m, nil
}

func (s *ProviderService) GetModel(ctx context.Context, id kernel.ModelID) (*provider.Model, error) {
	return s.modelRepo.FindByID(ctx, id)
}

func (s *ProviderService) ListModels(ctx context.Context) (*provider.ModelListResponse, error) {
	models, err := s.modelRepo.FindAll(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list models", errx.TypeInternal)
	}

	dtos := make([]provider.ModelDTO, len(models))
	for i, m := range models {
		dtos[i] = m.ToDTO()
	}
	return &provider.ModelListResponse{Models: dtos, Total: len(dtos)}, nil
}

func (s *ProviderService) ListActiveModels(ctx context.Context) (*provider.ModelListResponse, error) {
	models, err := s.modelRepo.FindActive(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list active models", errx.TypeInternal)
	}

	dtos := make([]provider.ModelDTO, len(models))
	for i, m := range models {
		dtos[i] = m.ToDTO()
	}
	return &provider.ModelListResponse{Models: dtos, Total: len(dtos)}, nil
}

func (s *ProviderService) UpdateModel(ctx context.Context, id kernel.ModelID, req provider.UpdateModelRequest) (*provider.Model, error) {
	m, err := s.modelRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		m.Name = *req.Name
	}
	if req.Description != nil {
		m.Description = *req.Description
	}
	if req.Family != nil {
		m.Family = *req.Family
	}
	if req.Stability != nil {
		m.Stability = *req.Stability
	}
	if req.Status != nil {
		m.Status = *req.Status
	}
	if req.Free != nil {
		m.Free = *req.Free
	}
	m.UpdatedAt = time.Now().UTC()

	if err := s.modelRepo.Save(ctx, *m); err != nil {
		return nil, errx.Wrap(err, "failed to update model", errx.TypeInternal)
	}
	return m, nil
}

func (s *ProviderService) DeleteModel(ctx context.Context, id kernel.ModelID) error {
	return s.modelRepo.Delete(ctx, id)
}

// GetModelWithMappings returns a model along with all its provider mappings
func (s *ProviderService) GetModelWithMappings(ctx context.Context, modelID kernel.ModelID) (*provider.ModelWithMappings, error) {
	m, err := s.modelRepo.FindByID(ctx, modelID)
	if err != nil {
		return nil, err
	}

	mappings, err := s.mappingRepo.FindByModel(ctx, modelID)
	if err != nil {
		return nil, errx.Wrap(err, "failed to get mappings for model", errx.TypeInternal)
	}

	mappingDTOs := make([]provider.ModelProviderMappingDTO, len(mappings))
	for i, mp := range mappings {
		mappingDTOs[i] = mp.ToDTO()
	}

	return &provider.ModelWithMappings{
		Model:    m.ToDTO(),
		Mappings: mappingDTOs,
	}, nil
}

// ============================================================================
// Mapping operations
// ============================================================================

func (s *ProviderService) CreateMapping(ctx context.Context, req provider.CreateMappingRequest) (*provider.ModelProviderMapping, error) {
	// Verify provider exists
	if _, err := s.providerRepo.FindByID(ctx, req.ProviderID); err != nil {
		return nil, err
	}
	// Verify model exists
	if _, err := s.modelRepo.FindByID(ctx, req.ModelID); err != nil {
		return nil, err
	}

	var region *string
	if req.Region != "" {
		region = &req.Region
	}

	m := &provider.ModelProviderMapping{
		ID:               kernel.NewMappingID(uuid.NewString()),
		ModelID:          req.ModelID,
		ProviderID:       req.ProviderID,
		ExternalID:       req.ExternalID,
		InputPrice:       req.InputPrice,
		OutputPrice:      req.OutputPrice,
		CachedInputPrice: req.CachedInputPrice,
		RequestPrice:     req.RequestPrice,
		ImageInputPrice:  req.ImageInputPrice,
		ContextSize:      req.ContextSize,
		MaxOutput:        req.MaxOutput,
		Streaming:        req.Streaming,
		Vision:           req.Vision,
		Reasoning:        req.Reasoning,
		Tools:            req.Tools,
		JSONOutput:       req.JSONOutput,
		Region:           region,
		Stability:        provider.StabilityStable,
		Status:           provider.ModelStatusActive,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := s.mappingRepo.Save(ctx, *m); err != nil {
		return nil, errx.Wrap(err, "failed to save mapping", errx.TypeInternal)
	}
	return m, nil
}

func (s *ProviderService) GetMapping(ctx context.Context, id kernel.MappingID) (*provider.ModelProviderMapping, error) {
	return s.mappingRepo.FindByID(ctx, id)
}

func (s *ProviderService) ListMappingsByModel(ctx context.Context, modelID kernel.ModelID) (*provider.MappingListResponse, error) {
	mappings, err := s.mappingRepo.FindByModel(ctx, modelID)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list mappings", errx.TypeInternal)
	}

	dtos := make([]provider.ModelProviderMappingDTO, len(mappings))
	for i, m := range mappings {
		dtos[i] = m.ToDTO()
	}
	return &provider.MappingListResponse{Mappings: dtos, Total: len(dtos)}, nil
}

func (s *ProviderService) ListMappingsByProvider(ctx context.Context, providerID kernel.ProviderID) (*provider.MappingListResponse, error) {
	mappings, err := s.mappingRepo.FindByProvider(ctx, providerID)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list mappings", errx.TypeInternal)
	}

	dtos := make([]provider.ModelProviderMappingDTO, len(mappings))
	for i, m := range mappings {
		dtos[i] = m.ToDTO()
	}
	return &provider.MappingListResponse{Mappings: dtos, Total: len(dtos)}, nil
}

func (s *ProviderService) UpdateMapping(ctx context.Context, id kernel.MappingID, req provider.UpdateMappingRequest) (*provider.ModelProviderMapping, error) {
	m, err := s.mappingRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.ExternalID != nil {
		m.ExternalID = *req.ExternalID
	}
	if req.InputPrice != nil {
		m.InputPrice = req.InputPrice
	}
	if req.OutputPrice != nil {
		m.OutputPrice = req.OutputPrice
	}
	if req.CachedInputPrice != nil {
		m.CachedInputPrice = req.CachedInputPrice
	}
	if req.RequestPrice != nil {
		m.RequestPrice = req.RequestPrice
	}
	if req.ImageInputPrice != nil {
		m.ImageInputPrice = req.ImageInputPrice
	}
	if req.ContextSize != nil {
		m.ContextSize = req.ContextSize
	}
	if req.MaxOutput != nil {
		m.MaxOutput = req.MaxOutput
	}
	if req.Streaming != nil {
		m.Streaming = *req.Streaming
	}
	if req.Vision != nil {
		m.Vision = *req.Vision
	}
	if req.Reasoning != nil {
		m.Reasoning = *req.Reasoning
	}
	if req.Tools != nil {
		m.Tools = *req.Tools
	}
	if req.JSONOutput != nil {
		m.JSONOutput = *req.JSONOutput
	}
	if req.Region != nil {
		m.Region = req.Region
	}
	if req.Status != nil {
		m.Status = *req.Status
	}
	m.UpdatedAt = time.Now().UTC()

	if err := s.mappingRepo.Save(ctx, *m); err != nil {
		return nil, errx.Wrap(err, "failed to update mapping", errx.TypeInternal)
	}
	return m, nil
}

func (s *ProviderService) DeleteMapping(ctx context.Context, id kernel.MappingID) error {
	return s.mappingRepo.Delete(ctx, id)
}
