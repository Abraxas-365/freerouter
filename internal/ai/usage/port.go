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
}
