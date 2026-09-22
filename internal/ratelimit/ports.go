package ratelimit

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// ── Commands ────────────────────────────────────────────────────────

// Commands defines write operations for rate limit configs.
type Commands interface {
	Create(ctx context.Context, cmd CreateRateLimitConfig) (RateLimitConfig, error)
	Update(ctx context.Context, id identity.RateLimitConfigID, cmd UpdateRateLimitConfig) (RateLimitConfig, error)
	Delete(ctx context.Context, id identity.RateLimitConfigID) error
}

// ── Queries ─────────────────────────────────────────────────────────

// Queries defines read operations for rate limit configs.
type Queries interface {
	Find(ctx context.Context, id identity.RateLimitConfigID) (RateLimitConfig, error)
	FindBySubject(ctx context.Context, subjectID string) (RateLimitConfig, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[RateLimitConfig], error)
}

// ── Repository ──────────────────────────────────────────────────────

// Repository is the persistence contract for rate limit configs.
type Repository interface {
	Create(ctx context.Context, cfg RateLimitConfig) error
	Find(ctx context.Context, id identity.RateLimitConfigID) (RateLimitConfig, error)
	FindBySubject(ctx context.Context, subjectID string) (RateLimitConfig, error)
	Update(ctx context.Context, cfg RateLimitConfig) error
	Delete(ctx context.Context, id identity.RateLimitConfigID) error
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[RateLimitConfig], error)
}

// ── Limiter ─────────────────────────────────────────────────────────

// Limiter enforces real-time rate limits (RPM + concurrency) via an
// external store (Redis). It does NOT manage config CRUD.
type Limiter interface {
	// Check verifies RPM and acquires a concurrency slot atomically.
	// On success the caller MUST call Release when the request finishes.
	Check(ctx context.Context, subjectID string, rpm, maxConcurrent int) (*RateLimitResult, error)

	// Release frees the concurrency slot for the subject.
	Release(ctx context.Context, subjectID string)
}
