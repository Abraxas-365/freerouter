package routingconfig

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// ── Commands ────────────────────────────────────────────────────────

// Commands defines write operations for routing configs.
type Commands interface {
	Create(ctx context.Context, cmd CreateRoutingConfig) (RoutingConfig, error)
	Update(ctx context.Context, id identity.RoutingConfigID, cmd UpdateRoutingConfig) (RoutingConfig, error)
	Delete(ctx context.Context, id identity.RoutingConfigID) error
}

// ── Queries ─────────────────────────────────────────────────────────

// Queries defines read operations for routing configs.
type Queries interface {
	Find(ctx context.Context, id identity.RoutingConfigID) (RoutingConfig, error)
	FindBySubject(ctx context.Context, subjectID string) (RoutingConfig, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[RoutingConfig], error)
}

// ── Resolver ────────────────────────────────────────────────────────

// Resolver is the gateway-facing lookup used to determine the effective
// routing strategy for a subject. It never errors: on any lookup failure
// (no config, backend unavailable) it returns DefaultStrategy.
type Resolver interface {
	Resolve(ctx context.Context, subjectID string) Strategy
}

// ── Repository ──────────────────────────────────────────────────────

// Repository is the persistence contract for routing configs.
type Repository interface {
	Create(ctx context.Context, cfg RoutingConfig) error
	Find(ctx context.Context, id identity.RoutingConfigID) (RoutingConfig, error)
	FindBySubject(ctx context.Context, subjectID string) (RoutingConfig, error)
	Update(ctx context.Context, cfg RoutingConfig) error
	Delete(ctx context.Context, id identity.RoutingConfigID) error
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[RoutingConfig], error)
}
