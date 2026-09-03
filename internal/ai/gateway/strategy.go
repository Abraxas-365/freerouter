package gateway

import (
	"context"
	"math/rand"
	"sort"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// RoutingStrategy determines how routes are ordered when multiple providers
// are available for a model.
type RoutingStrategy string

const (
	// StrategyCheapest picks the cheapest provider (by output price).
	// This is the default.
	StrategyCheapest RoutingStrategy = "cheapest"

	// StrategyLowestLatency picks the provider with the lowest recent
	// average latency (tracked by KeyHealthTracker).
	StrategyLowestLatency RoutingStrategy = "lowest-latency"

	// StrategyRoundRobin distributes requests evenly across providers
	// using a simple random shuffle.
	StrategyRoundRobin RoutingStrategy = "round-robin"
)

// ValidRoutingStrategies lists all valid strategy values.
var ValidRoutingStrategies = []RoutingStrategy{
	StrategyCheapest,
	StrategyLowestLatency,
	StrategyRoundRobin,
}

// IsValid returns true if the strategy is a recognized value.
func (s RoutingStrategy) IsValid() bool {
	for _, v := range ValidRoutingStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// RoutingConfig is a per-tenant routing configuration stored in the DB.
type RoutingConfig struct {
	TenantID kernel.TenantID `json:"tenant_id" db:"tenant_id"`
	Strategy RoutingStrategy `json:"strategy" db:"strategy"`
}

// RoutingConfigRepository persists per-tenant routing strategy preferences.
type RoutingConfigRepository interface {
	GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*RoutingConfig, error)
	Upsert(ctx context.Context, cfg *RoutingConfig) (*RoutingConfig, error)
	Delete(ctx context.Context, tenantID kernel.TenantID) error
}

// UpsertRoutingConfigRequest is the DTO for creating/updating routing config.
type UpsertRoutingConfigRequest struct {
	Strategy string `json:"strategy"`
}

// applyStrategy reorders routes according to the given strategy.
// Health filtering has already been applied; this only changes ordering.
func applyStrategy(routes []*RouteResult, strategy RoutingStrategy, ht *KeyHealthTracker) {
	if len(routes) <= 1 {
		return
	}

	switch strategy {
	case StrategyLowestLatency:
		sortByLatency(routes, ht)
	case StrategyRoundRobin:
		shuffleRoutes(routes)
	case StrategyCheapest:
		sortByCost(routes)
	default:
		sortByCost(routes)
	}
}

// sortByCost orders routes by output price ascending (cheapest first).
// Routes without pricing go last.
func sortByCost(routes []*RouteResult) {
	sort.SliceStable(routes, func(i, j int) bool {
		pi, pj := routes[i].OutputPrice, routes[j].OutputPrice
		if pi == nil && pj == nil {
			return false
		}
		if pi == nil {
			return false
		}
		if pj == nil {
			return true
		}
		return *pi < *pj
	})
}

// sortByLatency orders routes by recent average latency (fastest first).
// Keys with no latency data go last.
func sortByLatency(routes []*RouteResult, ht *KeyHealthTracker) {
	sort.SliceStable(routes, func(i, j int) bool {
		li := ht.AverageLatency(routes[i].KeyID)
		lj := ht.AverageLatency(routes[j].KeyID)
		if li == 0 && lj == 0 {
			return false
		}
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
}

// shuffleRoutes randomly shuffles routes for round-robin distribution.
func shuffleRoutes(routes []*RouteResult) {
	rand.Shuffle(len(routes), func(i, j int) {
		routes[i], routes[j] = routes[j], routes[i]
	})
}
