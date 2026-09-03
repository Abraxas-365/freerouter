package gatewayinfra

import (
	"context"
	"database/sql"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/jmoiron/sqlx"
)

type PostgresRateLimitConfigRepository struct {
	db *sqlx.DB
}

func NewPostgresRateLimitConfigRepository(db *sqlx.DB) gateway.RateLimitConfigRepository {
	return &PostgresRateLimitConfigRepository{db: db}
}

func (r *PostgresRateLimitConfigRepository) GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*gateway.RateLimitConfig, error) {
	var cfg gateway.RateLimitConfig
	err := r.db.GetContext(ctx, &cfg, `SELECT tenant_id, rpm, max_concurrent FROM rate_limit_configs WHERE tenant_id = $1`, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *PostgresRateLimitConfigRepository) Upsert(ctx context.Context, cfg *gateway.RateLimitConfig) (*gateway.RateLimitConfig, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO rate_limit_configs (tenant_id, rpm, max_concurrent, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			rpm = EXCLUDED.rpm,
			max_concurrent = EXCLUDED.max_concurrent,
			updated_at = NOW()
	`, cfg.TenantID, cfg.RPM, cfg.MaxConcurrent)
	if err != nil {
		return nil, err
	}
	return r.GetByTenantID(ctx, cfg.TenantID)
}

func (r *PostgresRateLimitConfigRepository) Delete(ctx context.Context, tenantID kernel.TenantID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rate_limit_configs WHERE tenant_id = $1`, tenantID)
	return err
}

// ============================================================================
// Routing Config Repository
// ============================================================================

type PostgresRoutingConfigRepository struct {
	db *sqlx.DB
}

func NewPostgresRoutingConfigRepository(db *sqlx.DB) gateway.RoutingConfigRepository {
	return &PostgresRoutingConfigRepository{db: db}
}

func (r *PostgresRoutingConfigRepository) GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*gateway.RoutingConfig, error) {
	var cfg gateway.RoutingConfig
	err := r.db.GetContext(ctx, &cfg, `SELECT tenant_id, strategy FROM routing_configs WHERE tenant_id = $1`, tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *PostgresRoutingConfigRepository) Upsert(ctx context.Context, cfg *gateway.RoutingConfig) (*gateway.RoutingConfig, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO routing_configs (tenant_id, strategy, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			strategy = EXCLUDED.strategy,
			updated_at = NOW()
	`, cfg.TenantID, cfg.Strategy)
	if err != nil {
		return nil, err
	}
	return r.GetByTenantID(ctx, cfg.TenantID)
}

func (r *PostgresRoutingConfigRepository) Delete(ctx context.Context, tenantID kernel.TenantID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM routing_configs WHERE tenant_id = $1`, tenantID)
	return err
}
