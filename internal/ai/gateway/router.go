package gateway

import (
	"context"
	"fmt"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// Router resolves a model request into a concrete provider + credential
type Router struct {
	modelRepo    provider.ModelRepository
	mappingRepo  provider.MappingRepository
	providerRepo provider.ProviderRepository
	keyRepo      providerkey.ProviderKeyRepository
	encryptor    providerkey.TokenEncryptor
	billingRepo  billing.BillingRepository
}

func NewRouter(
	modelRepo provider.ModelRepository,
	mappingRepo provider.MappingRepository,
	providerRepo provider.ProviderRepository,
	keyRepo providerkey.ProviderKeyRepository,
	encryptor providerkey.TokenEncryptor,
	billingRepo billing.BillingRepository,
) *Router {
	return &Router{
		modelRepo:    modelRepo,
		mappingRepo:  mappingRepo,
		providerRepo: providerRepo,
		keyRepo:      keyRepo,
		encryptor:    encryptor,
		billingRepo:  billingRepo,
	}
}

// Resolve finds the best provider + credential for the requested model.
// It returns a RouteResult with all the info needed to make the upstream call.
// maxTokens is the client's requested max output tokens (used for cost estimation).
func (r *Router) Resolve(ctx context.Context, modelID string, tenantID *kernel.TenantID, maxTokens *int) (*RouteResult, error) {
	// 1. Find the model
	model, err := r.modelRepo.FindByID(ctx, kernel.NewModelID(modelID))
	if err != nil {
		return nil, errx.New(
			fmt.Sprintf("model %q not found", modelID),
			errx.TypeNotFound,
		).WithDetail("model", modelID)
	}
	if !model.IsActive() {
		return nil, errx.New(
			fmt.Sprintf("model %q is inactive", modelID),
			errx.TypeBusiness,
		).WithDetail("model", modelID)
	}

	// 2. Find active provider mappings for this model
	mappings, err := r.mappingRepo.FindActiveByModel(ctx, model.ID)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find provider mappings", errx.TypeInternal)
	}
	if len(mappings) == 0 {
		return nil, errx.New(
			fmt.Sprintf("no active providers for model %q", modelID),
			errx.TypeNotFound,
		).WithDetail("model", modelID)
	}

	// 3. Pre-check: estimate max cost from the cheapest mapping and reject if
	//    the tenant can't afford even one request. Uses output_price * max_tokens
	//    as a conservative estimate (output tokens are the expensive part).
	if tenantID != nil && !tenantID.IsEmpty() {
		balance, err := r.billingRepo.GetBalance(ctx, *tenantID)
		if err != nil {
			return nil, errx.Wrap(err, "failed to check balance", errx.TypeInternal)
		}
		if balance.Balance <= 0 {
			return nil, billing.ErrInsufficientBalance().
				WithDetail("balance", fmt.Sprintf("%.6f", balance.Balance))
		}

		// Estimate cost if we have pricing info
		estimatedTokens := 4096
		if maxTokens != nil && *maxTokens > 0 {
			estimatedTokens = *maxTokens
		}
		if bestPrice := cheapestOutputPrice(mappings); bestPrice > 0 {
			estimatedCost := float64(estimatedTokens) * bestPrice / 1_000_000
			if balance.Balance < estimatedCost {
				return nil, billing.ErrInsufficientBalance().
					WithDetail("balance", fmt.Sprintf("%.6f", balance.Balance)).
					WithDetail("estimated_cost", fmt.Sprintf("%.6f", estimatedCost))
			}
		}
	}

	// 4. Try each mapping until we find one with an available credential
	for _, mapping := range mappings {
		// Check provider is active
		prov, err := r.providerRepo.FindByID(ctx, mapping.ProviderID)
		if err != nil || !prov.IsActive() {
			continue
		}

		// Find a credential: prefer BYOK for this tenant, fallback to managed
		key, err := r.resolveCredential(ctx, mapping.ProviderID, tenantID)
		if err != nil {
			continue
		}

		// Decrypt the token
		token, err := r.encryptor.Decrypt(key.TokenCiphertext)
		if err != nil {
			continue
		}

		// Build the base URL
		baseURL := defaultBaseURL(prov.ID.String())
		if key.BaseURL != nil && *key.BaseURL != "" {
			baseURL = *key.BaseURL
		}

		return &RouteResult{
			ProviderID:  prov.ID,
			ExternalID:  mapping.ExternalID,
			MappingID:   mapping.ID,
			Token:       token,
			BaseURL:     baseURL,
			KeyID:       key.ID,
			InputPrice:  mapping.InputPrice,
			OutputPrice: mapping.OutputPrice,
		}, nil
	}

	return nil, errx.New(
		fmt.Sprintf("no available credentials for model %q", modelID),
		errx.TypeBusiness,
	).WithDetail("model", modelID)
}

// resolveCredential finds the best credential for a provider.
// Priority: tenant BYOK key > platform managed key.
func (r *Router) resolveCredential(ctx context.Context, providerID kernel.ProviderID, tenantID *kernel.TenantID) (*providerkey.ProviderKey, error) {
	// Try tenant's BYOK keys first
	if tenantID != nil && !tenantID.IsEmpty() {
		keys, err := r.keyRepo.FindByTenant(ctx, *tenantID)
		if err == nil {
			for _, k := range keys {
				if k.ProviderID == providerID && k.IsActive() {
					return k, nil
				}
			}
		}
	}

	// Fallback to managed (platform) keys
	keys, err := r.keyRepo.FindActiveByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.IsManaged() {
			return k, nil
		}
	}

	return nil, fmt.Errorf("no credential found for provider %s", providerID)
}

// cheapestOutputPrice returns the lowest output price from a set of mappings.
func cheapestOutputPrice(mappings []*provider.ModelProviderMapping) float64 {
	var best float64
	for _, m := range mappings {
		if m.OutputPrice != nil {
			if best == 0 || *m.OutputPrice < best {
				best = *m.OutputPrice
			}
		}
	}
	return best
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
