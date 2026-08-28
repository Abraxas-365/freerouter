package webhookinfra

import (
	"context"
	"database/sql"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type PostgresWebhookRepository struct {
	db *sqlx.DB
}

func NewPostgresWebhookRepository(db *sqlx.DB) webhook.WebhookRepository {
	return &PostgresWebhookRepository{db: db}
}

func (r *PostgresWebhookRepository) FindByID(ctx context.Context, id kernel.WebhookID) (*webhook.WebhookConfig, error) {
	var w webhookRow
	err := r.db.GetContext(ctx, &w, `SELECT id, tenant_id, url, secret, events, enabled, created_at, updated_at FROM webhook_configs WHERE id = $1`, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, webhook.ErrWebhookNotFound()
		}
		return nil, errx.Wrap(err, "failed to find webhook", errx.TypeInternal)
	}
	return w.toEntity(), nil
}

func (r *PostgresWebhookRepository) FindByTenant(ctx context.Context, tenantID kernel.TenantID) ([]*webhook.WebhookConfig, error) {
	var rows []webhookRow
	err := r.db.SelectContext(ctx, &rows, `SELECT id, tenant_id, url, secret, events, enabled, created_at, updated_at FROM webhook_configs WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find webhooks", errx.TypeInternal)
	}
	result := make([]*webhook.WebhookConfig, len(rows))
	for i, row := range rows {
		result[i] = row.toEntity()
	}
	return result, nil
}

func (r *PostgresWebhookRepository) FindEnabledByTenantAndEvent(ctx context.Context, tenantID kernel.TenantID, event string) ([]*webhook.WebhookConfig, error) {
	var rows []webhookRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, tenant_id, url, secret, events, enabled, created_at, updated_at
		FROM webhook_configs
		WHERE tenant_id = $1 AND enabled = true AND $2 = ANY(events)
	`, tenantID.String(), event)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find webhooks for event", errx.TypeInternal)
	}
	result := make([]*webhook.WebhookConfig, len(rows))
	for i, row := range rows {
		result[i] = row.toEntity()
	}
	return result, nil
}

func (r *PostgresWebhookRepository) Save(ctx context.Context, w *webhook.WebhookConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO webhook_configs (id, tenant_id, url, secret, events, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			url = EXCLUDED.url,
			secret = EXCLUDED.secret,
			events = EXCLUDED.events,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()
	`, w.ID.String(), w.TenantID.String(), w.URL, w.Secret, pq.Array(w.Events), w.Enabled, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return errx.Wrap(err, "failed to save webhook", errx.TypeInternal)
	}
	return nil
}

func (r *PostgresWebhookRepository) Delete(ctx context.Context, id kernel.WebhookID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM webhook_configs WHERE id = $1`, id.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete webhook", errx.TypeInternal)
	}
	return nil
}

// ============================================================================
// Delivery methods
// ============================================================================

func (r *PostgresWebhookRepository) SaveDelivery(ctx context.Context, d *webhook.WebhookDelivery) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status, status_code, attempts, last_error, next_retry_at, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, d.ID.String(), d.WebhookID.String(), d.EventType, d.Payload, d.Status, d.StatusCode, d.Attempts, d.LastError, d.NextRetryAt, d.CreatedAt, d.CompletedAt)
	if err != nil {
		return errx.Wrap(err, "failed to save delivery", errx.TypeInternal)
	}
	return nil
}

func (r *PostgresWebhookRepository) UpdateDelivery(ctx context.Context, d *webhook.WebhookDelivery) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_deliveries SET
			status = $2, status_code = $3, attempts = $4, last_error = $5,
			next_retry_at = $6, completed_at = $7
		WHERE id = $1
	`, d.ID.String(), d.Status, d.StatusCode, d.Attempts, d.LastError, d.NextRetryAt, d.CompletedAt)
	if err != nil {
		return errx.Wrap(err, "failed to update delivery", errx.TypeInternal)
	}
	return nil
}

func (r *PostgresWebhookRepository) FindPendingDeliveries(ctx context.Context, limit int) ([]*webhook.WebhookDelivery, error) {
	var deliveries []*webhook.WebhookDelivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT id, webhook_id, event_type, payload, status, status_code, attempts, last_error, next_retry_at, created_at, completed_at
		FROM webhook_deliveries
		WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find pending deliveries", errx.TypeInternal)
	}
	return deliveries, nil
}

func (r *PostgresWebhookRepository) FindDeliveriesByWebhook(ctx context.Context, webhookID kernel.WebhookID, limit int) ([]*webhook.WebhookDelivery, error) {
	var deliveries []*webhook.WebhookDelivery
	err := r.db.SelectContext(ctx, &deliveries, `
		SELECT id, webhook_id, event_type, payload, status, status_code, attempts, last_error, next_retry_at, created_at, completed_at
		FROM webhook_deliveries
		WHERE webhook_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, webhookID.String(), limit)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find deliveries", errx.TypeInternal)
	}
	return deliveries, nil
}

// ============================================================================
// Row type for Postgres array scanning
// ============================================================================

type webhookRow struct {
	ID        string         `db:"id"`
	TenantID  string         `db:"tenant_id"`
	URL       string         `db:"url"`
	Secret    string         `db:"secret"`
	Events    pq.StringArray `db:"events"`
	Enabled   bool           `db:"enabled"`
	CreatedAt sql.NullTime   `db:"created_at"`
	UpdatedAt sql.NullTime   `db:"updated_at"`
}

func (r *webhookRow) toEntity() *webhook.WebhookConfig {
	w := &webhook.WebhookConfig{
		ID:       kernel.NewWebhookID(r.ID),
		TenantID: kernel.NewTenantID(r.TenantID),
		URL:      r.URL,
		Secret:   r.Secret,
		Events:   []string(r.Events),
		Enabled:  r.Enabled,
	}
	if r.CreatedAt.Valid {
		w.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		w.UpdatedAt = r.UpdatedAt.Time
	}
	return w
}
