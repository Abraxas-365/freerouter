package usage

import (
	"context"
	"time"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// UsageRepository defines the contract for usage log persistence
type UsageRepository interface {
	Create(ctx context.Context, log UsageLog) error
	FindByID(ctx context.Context, id kernel.UsageLogID) (*UsageLog, error)
	Query(ctx context.Context, q UsageQuery) ([]*UsageLog, int, error)
	GetSummary(ctx context.Context, tenantID kernel.TenantID, from, to *time.Time) (*UsageSummary, error)
	GetSummaryByModel(ctx context.Context, tenantID kernel.TenantID, from, to *time.Time) ([]ModelUsageSummary, error)
	NullifyExpiredContent(ctx context.Context, before time.Time) (int, error)
}

// DataRetentionRepository defines the contract for per-tenant data retention config persistence.
type DataRetentionRepository interface {
	GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*DataRetentionConfig, error)
	Upsert(ctx context.Context, cfg DataRetentionConfig) (*DataRetentionConfig, error)
	Delete(ctx context.Context, tenantID kernel.TenantID) error
	ListAll(ctx context.Context) ([]*DataRetentionConfig, error)
}
