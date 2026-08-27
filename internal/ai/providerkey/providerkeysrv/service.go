package providerkeysrv

import (
	"context"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/google/uuid"
)

type ProviderKeyService struct {
	keyRepo      providerkey.ProviderKeyRepository
	providerRepo provider.ProviderRepository
	encryptor    providerkey.TokenEncryptor
}

func NewProviderKeyService(
	keyRepo providerkey.ProviderKeyRepository,
	providerRepo provider.ProviderRepository,
	encryptor providerkey.TokenEncryptor,
) *ProviderKeyService {
	return &ProviderKeyService{
		keyRepo:      keyRepo,
		providerRepo: providerRepo,
		encryptor:    encryptor,
	}
}

func (s *ProviderKeyService) CreateKey(ctx context.Context, req providerkey.CreateProviderKeyRequest) (*providerkey.ProviderKey, error) {
	// Verify provider exists
	if _, err := s.providerRepo.FindByID(ctx, req.ProviderID); err != nil {
		return nil, err
	}

	ciphertext, err := s.encryptor.Encrypt(req.Token)
	if err != nil {
		return nil, errx.Wrap(err, "failed to encrypt token", errx.TypeInternal)
	}

	managed := req.TenantID == nil || req.TenantID.IsEmpty()

	k := &providerkey.ProviderKey{
		ID:              kernel.NewProviderKeyID(uuid.NewString()),
		ProviderID:      req.ProviderID,
		TenantID:        req.TenantID,
		TokenCiphertext: ciphertext,
		TokenMasked:     s.encryptor.Mask(req.Token),
		TokenHash:       s.encryptor.Hash(req.Token),
		BaseURL:         req.BaseURL,
		Name:            req.Name,
		Description:     req.Description,
		Managed:         managed,
		Status:          providerkey.KeyStatusActive,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.keyRepo.Save(ctx, *k); err != nil {
		return nil, errx.Wrap(err, "failed to save provider key", errx.TypeInternal)
	}
	return k, nil
}

func (s *ProviderKeyService) GetKey(ctx context.Context, id kernel.ProviderKeyID) (*providerkey.ProviderKey, error) {
	return s.keyRepo.FindByID(ctx, id)
}

func (s *ProviderKeyService) ListByProvider(ctx context.Context, providerID kernel.ProviderID) (*providerkey.ProviderKeyListResponse, error) {
	keys, err := s.keyRepo.FindByProvider(ctx, providerID)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list keys by provider", errx.TypeInternal)
	}

	dtos := make([]providerkey.ProviderKeyDTO, len(keys))
	for i, k := range keys {
		dtos[i] = k.ToDTO()
	}
	return &providerkey.ProviderKeyListResponse{Keys: dtos, Total: len(dtos)}, nil
}

func (s *ProviderKeyService) ListByTenant(ctx context.Context, tenantID kernel.TenantID) (*providerkey.ProviderKeyListResponse, error) {
	keys, err := s.keyRepo.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list keys by tenant", errx.TypeInternal)
	}

	dtos := make([]providerkey.ProviderKeyDTO, len(keys))
	for i, k := range keys {
		dtos[i] = k.ToDTO()
	}
	return &providerkey.ProviderKeyListResponse{Keys: dtos, Total: len(dtos)}, nil
}

func (s *ProviderKeyService) ListManaged(ctx context.Context) (*providerkey.ProviderKeyListResponse, error) {
	keys, err := s.keyRepo.FindManaged(ctx)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list managed keys", errx.TypeInternal)
	}

	dtos := make([]providerkey.ProviderKeyDTO, len(keys))
	for i, k := range keys {
		dtos[i] = k.ToDTO()
	}
	return &providerkey.ProviderKeyListResponse{Keys: dtos, Total: len(dtos)}, nil
}

func (s *ProviderKeyService) UpdateKey(ctx context.Context, id kernel.ProviderKeyID, req providerkey.UpdateProviderKeyRequest) (*providerkey.ProviderKey, error) {
	k, err := s.keyRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Token != nil {
		ciphertext, err := s.encryptor.Encrypt(*req.Token)
		if err != nil {
			return nil, errx.Wrap(err, "failed to encrypt token", errx.TypeInternal)
		}
		k.TokenCiphertext = ciphertext
		k.TokenMasked = s.encryptor.Mask(*req.Token)
		k.TokenHash = s.encryptor.Hash(*req.Token)
	}
	if req.BaseURL != nil {
		k.BaseURL = req.BaseURL
	}
	if req.Name != nil {
		k.Name = *req.Name
	}
	if req.Description != nil {
		k.Description = *req.Description
	}
	if req.Status != nil {
		k.Status = *req.Status
	}
	if req.SortOrder != nil {
		k.SortOrder = req.SortOrder
	}
	k.UpdatedAt = time.Now().UTC()

	if err := s.keyRepo.Save(ctx, *k); err != nil {
		return nil, errx.Wrap(err, "failed to update provider key", errx.TypeInternal)
	}
	return k, nil
}

func (s *ProviderKeyService) DeleteKey(ctx context.Context, id kernel.ProviderKeyID) error {
	return s.keyRepo.Delete(ctx, id)
}

// DecryptToken retrieves and decrypts the token for a provider key (used by the gateway at runtime)
func (s *ProviderKeyService) DecryptToken(ctx context.Context, id kernel.ProviderKeyID) (string, error) {
	k, err := s.keyRepo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}

	plaintext, err := s.encryptor.Decrypt(k.TokenCiphertext)
	if err != nil {
		return "", errx.Wrap(err, "failed to decrypt token", errx.TypeInternal)
	}
	return plaintext, nil
}
