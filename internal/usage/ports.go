package usage

import (
	"context"
	"time"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// Commands defines write operations for usage logs and retention config.
type Commands interface {
	// LogRequest is the primary entry point — non-blocking, enqueues for async persistence.
	LogRequest(log UsageLog)

	UpsertRetention(ctx context.Context, cmd UpsertRetention) (RetentionConfig, error)
	DeleteRetention(ctx context.Context) error
}

// Queries defines read operations for usage logs and retention config.
type Queries interface {
	Find(ctx context.Context, id identity.UsageLogID) (UsageLog, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[UsageLog], error)
	GetSummary(ctx context.Context, from, to *time.Time) (*SummaryResponse, error)

	GetRetention(ctx context.Context) (RetentionConfig, error)
}

// Repository is the persistence contract for usage logs and retention config.
type Repository interface {
	Create(ctx context.Context, log UsageLog) error
	Find(ctx context.Context, id identity.UsageLogID) (UsageLog, error)
	List(ctx context.Context, filter Filter, page query.Pagination) (query.Paginated[UsageLog], error)
	GetSummary(ctx context.Context, from, to *time.Time) (*Summary, error)
	GetSummaryByModel(ctx context.Context, from, to *time.Time) ([]ModelSummary, error)

	GetRetention(ctx context.Context) (RetentionConfig, error)
	UpsertRetention(ctx context.Context, cfg RetentionConfig) error
	DeleteRetention(ctx context.Context) error
	PurgeOlderThan(ctx context.Context, days int) (int64, error)
}
