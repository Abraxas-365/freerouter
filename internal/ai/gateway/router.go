package gateway

import (
	"context"
	"fmt"
	"math"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// Router resolves a model request into a concrete provider + credential
type Router struct {
	modelRepo     provider.ModelRepository
	mappingRepo   provider.MappingRepository
	providerRepo  provider.ProviderRepository
	keyRepo       providerkey.ProviderKeyRepository
	encryptor     providerkey.TokenEncryptor
	billingRepo   billing.BillingRepository
	healthTracker *KeyHealthTracker
	fallbackRepo  provider.FallbackRepository // nil = no model fallback
}

func NewRouter(
	modelRepo provider.ModelRepository,
	mappingRepo provider.MappingRepository,
	providerRepo provider.ProviderRepository,
	keyRepo providerkey.ProviderKeyRepository,
	encryptor providerkey.TokenEncryptor,
	billingRepo billing.BillingRepository,
	healthTracker *KeyHealthTracker,
	fallbackRepo provider.FallbackRepository,
) *Router {
	return &Router{
		modelRepo:     modelRepo,
		mappingRepo:   mappingRepo,
		providerRepo:  providerRepo,
		keyRepo:       keyRepo,
		encryptor:     encryptor,
		billingRepo:   billingRepo,
		healthTracker: healthTracker,
		fallbackRepo:  fallbackRepo,
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

// ResolveAll returns all viable routes for a model, ordered by preference.
// Used by the retry/fallback system to try alternatives on failure.
// If model fallback is configured, routes for fallback models are appended
// after the primary model's routes.
func (r *Router) ResolveAll(ctx context.Context, modelID string, tenantID *kernel.TenantID, maxTokens *int) ([]*RouteResult, error) {
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

	// 3. Pre-check balance (same as Resolve)
	if tenantID != nil && !tenantID.IsEmpty() {
		balance, err := r.billingRepo.GetBalance(ctx, *tenantID)
		if err != nil {
			return nil, errx.Wrap(err, "failed to check balance", errx.TypeInternal)
		}
		if balance.Balance <= 0 {
			return nil, billing.ErrInsufficientBalance().
				WithDetail("balance", fmt.Sprintf("%.6f", balance.Balance))
		}

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

	// 4. Build routes for primary model
	routes := r.buildRoutesForMappings(ctx, mappings, tenantID, "", false)

	// 5. Append fallback model routes
	if r.fallbackRepo != nil {
		fallbacks, err := r.fallbackRepo.FindByModelID(ctx, model.ID)
		if err == nil {
			for _, fb := range fallbacks {
				fbMappings, err := r.mappingRepo.FindActiveByModel(ctx, fb.FallbackModelID)
				if err != nil || len(fbMappings) == 0 {
					continue
				}
				fbRoutes := r.buildRoutesForMappings(ctx, fbMappings, tenantID, fb.FallbackModelID.String(), true)
				routes = append(routes, fbRoutes...)
			}
		}
	}

	if len(routes) == 0 {
		return nil, errx.New(
			fmt.Sprintf("no available credentials for model %q", modelID),
			errx.TypeBusiness,
		).WithDetail("model", modelID)
	}

	return routes, nil
}

// ModelPricing holds pricing info for a model from its cheapest mapping.
type ModelPricing struct {
	ModelID      string   `json:"model_id"`
	ProviderID   string   `json:"provider_id"`
	InputPrice   *float64 `json:"input_price_per_million,omitempty"`
	OutputPrice  *float64 `json:"output_price_per_million,omitempty"`
	ContextSize  *int     `json:"context_size,omitempty"`
	MaxOutput    *int     `json:"max_output,omitempty"`
}

// GetPricing looks up the cheapest pricing for a model without resolving credentials.
func (r *Router) GetPricing(ctx context.Context, modelID string) (*ModelPricing, error) {
	model, err := r.modelRepo.FindByID(ctx, kernel.NewModelID(modelID))
	if err != nil {
		return nil, errx.New(fmt.Sprintf("model %q not found", modelID), errx.TypeNotFound)
	}

	mappings, err := r.mappingRepo.FindActiveByModel(ctx, model.ID)
	if err != nil || len(mappings) == 0 {
		return nil, errx.New(fmt.Sprintf("no active providers for model %q", modelID), errx.TypeNotFound)
	}

	// Pick the mapping with the lowest output price (cheapest)
	best := mappings[0]
	for _, m := range mappings[1:] {
		if m.OutputPrice != nil && (best.OutputPrice == nil || *m.OutputPrice < *best.OutputPrice) {
			best = m
		}
	}

	return &ModelPricing{
		ModelID:     model.ID.String(),
		ProviderID:  best.ProviderID.String(),
		InputPrice:  best.InputPrice,
		OutputPrice: best.OutputPrice,
		ContextSize: best.ContextSize,
		MaxOutput:   best.MaxOutput,
	}, nil
}

// buildRoutesForMappings builds routes for a set of mappings with all available credentials.
func (r *Router) buildRoutesForMappings(ctx context.Context, mappings []*provider.ModelProviderMapping, tenantID *kernel.TenantID, fallbackModelID string, isFallback bool) []*RouteResult {
	var routes []*RouteResult
	for _, mapping := range mappings {
		prov, err := r.providerRepo.FindByID(ctx, mapping.ProviderID)
		if err != nil || !prov.IsActive() {
			continue
		}

		keys := r.resolveAllCredentials(ctx, mapping.ProviderID, tenantID)
		for _, key := range keys {
			token, err := r.encryptor.Decrypt(key.TokenCiphertext)
			if err != nil {
				continue
			}

			baseURL := defaultBaseURL(prov.ID.String())
			if key.BaseURL != nil && *key.BaseURL != "" {
				baseURL = *key.BaseURL
			}

			routes = append(routes, &RouteResult{
				ProviderID:      prov.ID,
				ExternalID:      mapping.ExternalID,
				MappingID:       mapping.ID,
				Token:           token,
				BaseURL:         baseURL,
				KeyID:           key.ID,
				InputPrice:      mapping.InputPrice,
				OutputPrice:     mapping.OutputPrice,
				IsFallback:      isFallback,
				FallbackModelID: fallbackModelID,
			})
		}
	}
	return routes
}

// resolveAllCredentials returns all usable credentials for a provider,
// ordered by health (best first). Used by ResolveAll for retry/fallback.
func (r *Router) resolveAllCredentials(ctx context.Context, providerID kernel.ProviderID, tenantID *kernel.TenantID) []*providerkey.ProviderKey {
	var result []*providerkey.ProviderKey
	seen := make(map[kernel.ProviderKeyID]bool)

	// Tenant BYOK keys first
	if tenantID != nil && !tenantID.IsEmpty() {
		keys, err := r.keyRepo.FindByTenant(ctx, *tenantID)
		if err == nil {
			sorted := r.sortByHealth(keys, providerID, false)
			for _, k := range sorted {
				if !seen[k.ID] {
					result = append(result, k)
					seen[k.ID] = true
				}
			}
		}
	}

	// Managed keys
	keys, err := r.keyRepo.FindActiveByProvider(ctx, providerID)
	if err == nil {
		sorted := r.sortByHealth(keys, providerID, true)
		for _, k := range sorted {
			if !seen[k.ID] {
				result = append(result, k)
				seen[k.ID] = true
			}
		}
	}

	return result
}

// sortByHealth filters and sorts keys by health (healthiest first).
func (r *Router) sortByHealth(keys []*providerkey.ProviderKey, providerID kernel.ProviderID, managedOnly bool) []*providerkey.ProviderKey {
	type scored struct {
		key     *providerkey.ProviderKey
		healthy bool
		penalty float64
	}

	var candidates []scored
	for _, k := range keys {
		if k.ProviderID != providerID || !k.IsActive() {
			continue
		}
		if managedOnly && !k.IsManaged() {
			continue
		}
		metrics := r.healthTracker.GetMetrics(k.ID)
		if metrics.PermanentlyBlacklisted {
			continue
		}
		candidates = append(candidates, scored{
			key:     k,
			healthy: r.healthTracker.IsHealthy(k.ID),
			penalty: r.healthTracker.UptimePenalty(k.ID),
		})
	}

	// Sort: healthy first, then by penalty ascending
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0; j-- {
			a, b := candidates[j], candidates[j-1]
			swap := false
			if a.healthy && !b.healthy {
				swap = true
			} else if a.healthy == b.healthy && a.penalty < b.penalty {
				swap = true
			}
			if swap {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			} else {
				break
			}
		}
	}

	result := make([]*providerkey.ProviderKey, len(candidates))
	for i, c := range candidates {
		result[i] = c.key
	}
	return result
}

// resolveCredential finds the best credential for a provider.
// Priority: tenant BYOK key > platform managed key.
// Within each tier, the healthiest key is preferred.
func (r *Router) resolveCredential(ctx context.Context, providerID kernel.ProviderID, tenantID *kernel.TenantID) (*providerkey.ProviderKey, error) {
	// Try tenant's BYOK keys first (pick healthiest)
	if tenantID != nil && !tenantID.IsEmpty() {
		keys, err := r.keyRepo.FindByTenant(ctx, *tenantID)
		if err == nil {
			if best := r.pickHealthiest(keys, providerID, false); best != nil {
				return best, nil
			}
		}
	}

	// Fallback to managed (platform) keys (pick healthiest)
	keys, err := r.keyRepo.FindActiveByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if best := r.pickHealthiest(keys, providerID, true); best != nil {
		return best, nil
	}

	return nil, fmt.Errorf("no credential found for provider %s", providerID)
}

// pickHealthiest selects the best key from candidates considering health.
// Returns nil if no suitable key found (all permanently blacklisted or no matches).
func (r *Router) pickHealthiest(keys []*providerkey.ProviderKey, providerID kernel.ProviderID, managedOnly bool) *providerkey.ProviderKey {
	var best *providerkey.ProviderKey
	bestPenalty := math.Inf(1)
	bestHealthy := false

	for _, k := range keys {
		if k.ProviderID != providerID || !k.IsActive() {
			continue
		}
		if managedOnly && !k.IsManaged() {
			continue
		}

		metrics := r.healthTracker.GetMetrics(k.ID)
		if metrics.PermanentlyBlacklisted {
			continue
		}

		healthy := r.healthTracker.IsHealthy(k.ID)
		penalty := r.healthTracker.UptimePenalty(k.ID)

		// Prefer healthy over unhealthy, then lower penalty
		if best == nil ||
			(healthy && !bestHealthy) ||
			(healthy == bestHealthy && penalty < bestPenalty) {
			best = k
			bestPenalty = penalty
			bestHealthy = healthy
		}
	}

	return best
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
