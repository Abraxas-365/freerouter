package providerkeysrv

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// TestKeyResult contains the outcome of testing a provider key.
type TestKeyResult struct {
	Valid      bool   `json:"valid"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	LatencyMs  int64  `json:"latency_ms"`
}

// TestKey verifies a provider key by making a lightweight API call to the provider.
func (s *ProviderKeyService) TestKey(ctx context.Context, id kernel.ProviderKeyID) (*TestKeyResult, error) {
	k, err := s.keyRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	token, err := s.encryptor.Decrypt(k.TokenCiphertext)
	if err != nil {
		return nil, errx.Wrap(err, "failed to decrypt token", errx.TypeInternal)
	}

	prov, err := s.providerRepo.FindByID(ctx, k.ProviderID)
	if err != nil {
		return nil, err
	}

	baseURL := defaultBaseURL(prov.ID.String())
	if k.BaseURL != nil && *k.BaseURL != "" {
		baseURL = *k.BaseURL
	}

	if baseURL == "" {
		return &TestKeyResult{
			Valid:   false,
			Message: "no base URL configured for provider",
		}, nil
	}

	// Build a lightweight test request per provider
	_, testReq := buildTestRequest(prov.ID.String(), baseURL, token)
	if testReq == nil {
		return &TestKeyResult{
			Valid:   false,
			Message: "unsupported provider for key testing",
		}, nil
	}

	client := &http.Client{Timeout: 15 * time.Second}
	start := time.Now()
	resp, err := client.Do(testReq.WithContext(ctx))
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &TestKeyResult{
			Valid:     false,
			Message:   fmt.Sprintf("connection failed: %v", err),
			LatencyMs: latency,
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode == http.StatusOK {
		return &TestKeyResult{
			Valid:      true,
			StatusCode: resp.StatusCode,
			Message:    "key is valid",
			LatencyMs:  latency,
		}, nil
	}

	msg := fmt.Sprintf("provider returned %d", resp.StatusCode)
	if len(body) > 0 {
		msg = fmt.Sprintf("%s: %s", msg, strings.TrimSpace(string(body)))
	}

	return &TestKeyResult{
		Valid:      false,
		StatusCode: resp.StatusCode,
		Message:    msg,
		LatencyMs:  latency,
	}, nil
}

// buildTestRequest creates a lightweight request to validate an API key.
// Uses GET /models for OpenAI-compatible APIs and minimal POST for others.
func buildTestRequest(providerID, baseURL, token string) (string, *http.Request) {
	base := strings.TrimSuffix(baseURL, "/")

	switch providerID {
	case "anthropic":
		// Anthropic: POST /messages with minimal payload
		payload := `{"model":"claude-3-5-haiku-latest","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
		req, _ := http.NewRequest(http.MethodPost, base+"/messages", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", token)
		req.Header.Set("anthropic-version", "2023-06-01")
		return base + "/messages", req

	case "google", "google-ai-studio", "google-vertex":
		// Google: GET models list
		url := base + "/models?key=" + token
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		return url, req

	default:
		// OpenAI-compatible (openai, groq, together, mistral, deepseek, xai):
		// GET /models is the cheapest call
		url := base + "/models"
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		return url, req
	}
}

// defaultBaseURL returns the default API base URL for known providers.
func defaultBaseURL(providerID string) string {
	switch providerID {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com/v1"
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "together":
		return "https://api.together.xyz/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "xai":
		return "https://api.x.ai/v1"
	default:
		return ""
	}
}
