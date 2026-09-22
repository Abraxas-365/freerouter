package gateway

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
)

// Router resolves a model request into a concrete provider + credential.
type Router struct {
	models        provider.ModelQueries
	mappings      provider.MappingQueries
	providers     provider.Queries
	keys          providerkey.Queries
	encryptor     providerkey.TokenEncryptor
	healthTracker *KeyHealthTracker
	fallbacks     provider.FallbackQueries
}

// NewRouter creates a gateway router.
func NewRouter(
	models provider.ModelQueries,
	mappings provider.MappingQueries,
	providers provider.Queries,
	keys providerkey.Queries,
	encryptor providerkey.TokenEncryptor,
	healthTracker *KeyHealthTracker,
	fallbacks provider.FallbackQueries,
) *Router {
	return &Router{
		models:        models,
		mappings:      mappings,
		providers:     providers,
		keys:          keys,
		encryptor:     encryptor,
		healthTracker: healthTracker,
		fallbacks:     fallbacks,
	}
}

// Resolve finds a single viable route for a model name.
func (r *Router) Resolve(ctx context.Context, modelName string) (*RouteResult, error) {
	routes, err := r.ResolveAll(ctx, modelName, StrategyCheapest)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errx.NotFound("no available route for model: " + modelName)
	}
	return routes[0], nil
}

// ResolveAll returns all viable routes for a model (including fallbacks), ordered by strategy.
func (r *Router) ResolveAll(ctx context.Context, modelName string, strategy RoutingStrategy) ([]*RouteResult, error) {
	// Resolve model by name
	model, err := r.models.FindModelByName(ctx, modelName)
	if err != nil {
		return nil, err
	}
	if model.Status != provider.ModelStatusActive {
		return nil, errx.NotFound("model is not active: " + modelName)
	}

	// Get primary routes
	routes, err := r.routesForModel(ctx, model.ID, false)
	if err != nil {
		return nil, err
	}

	// Get fallback model routes
	fallbacks, err := r.fallbacks.ListFallbacks(ctx, model.ID)
	if err == nil {
		for _, fb := range fallbacks {
			if !fb.Enabled {
				continue
			}
			fbRoutes, fbErr := r.routesForModel(ctx, fb.FallbackModelID, true)
			if fbErr != nil {
				continue
			}
			routes = append(routes, fbRoutes...)
		}
	}

	ApplyStrategy(routes, strategy, r.healthTracker)
	return routes, nil
}

// routesForModel builds all routes for a single model ID.
func (r *Router) routesForModel(ctx context.Context, modelID identity.ModelID, isFallback bool) ([]*RouteResult, error) {
	mappings, err := r.mappings.ListActiveMappingsByModel(ctx, modelID)
	if err != nil {
		return nil, err
	}

	var routes []*RouteResult
	for _, mapping := range mappings {
		// Get provider
		prov, err := r.providers.Find(ctx, mapping.ProviderID)
		if err != nil || prov.Status != provider.ProviderStatusActive {
			continue
		}

		// Get active keys for this provider
		keys, err := r.keys.ListActiveByProvider(ctx, mapping.ProviderID)
		if err != nil || len(keys) == 0 {
			continue
		}

		// Pick the healthiest key
		key := r.pickBestKey(keys)
		if key == nil {
			continue
		}

		// Decrypt token
		token, err := r.keys.DecryptToken(ctx, key.ID)
		if err != nil {
			continue
		}

		// Determine base URL: key override > provider default > profile default
		baseURL := prov.BaseURL
		if key.BaseURL != nil && *key.BaseURL != "" {
			baseURL = *key.BaseURL
		}
		if baseURL == "" {
			baseURL = GetProfile(string(prov.Protocol)).DefaultBaseURL
		}

		routes = append(routes, &RouteResult{
			ProviderID:            mapping.ProviderID,
			Protocol:              string(prov.Protocol),
			KeyID:                 key.ID,
			KeyType:               string(key.KeyType),
			MappingID:             mapping.ID,
			ModelID:               mapping.ModelID,
			ExternalID:            mapping.ExternalID,
			Token:                 token,
			BaseURL:               baseURL,
			IsFallback:            isFallback,
			InputPrice:            mapping.InputPrice,
			OutputPrice:           mapping.OutputPrice,
			CachedInputPrice:      mapping.CachedInputPrice,
			AudioPricePerMinute:   mapping.AudioPricePerMinute,
			SpeechPricePer1kChars: mapping.SpeechPricePer1kChars,
			RerankPricePer1k:      mapping.RerankPricePer1k,
		})
	}

	return routes, nil
}

// pickBestKey selects the healthiest key from a set.
func (r *Router) pickBestKey(keys []providerkey.ProviderKey) *providerkey.ProviderKey {
	var best *providerkey.ProviderKey
	var bestPenalty float64
	var bestHealthy bool

	for i := range keys {
		k := &keys[i]
		healthy := r.healthTracker.IsHealthy(k.ID)
		penalty := r.healthTracker.UptimePenalty(k.ID)

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
