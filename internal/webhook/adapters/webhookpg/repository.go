package webhookpg

import (
	"context"
	"database/sql"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository implements webhook.Repository with PostgreSQL.
type Repository struct{ db *sqlx.DB }

var _ webhook.Repository = (*Repository)(nil)

// New creates a webhook repository.
func New(db *sqlx.DB) *Repository { return &Repository{db: db} }

// webhookRow mirrors WebhookConfig but scans Events as a pq.StringArray,
// since driver.Valuer/Scanner on []string doesn't handle Postgres arrays.
type webhookRow struct {
	ID        identity.WebhookID `db:"id"`
	URL       string             `db:"url"`
	Secret    string             `db:"secret"`
	Events    pq.StringArray     `db:"events"`
	Enabled   bool               `db:"enabled"`
	CreatedAt sql.NullTime       `db:"created_at"`
	UpdatedAt sql.NullTime       `db:"updated_at"`
}

func (r webhookRow) toEntity() webhook.WebhookConfig {
	return webhook.WebhookConfig{
		ID:        r.ID,
		URL:       r.URL,
		Secret:    r.Secret,
		Events:    []string(r.Events),
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}

// ── Config ──────────────────────────────────────────────────────────

func (r *Repository) Find(ctx context.Context, id identity.WebhookID) (webhook.WebhookConfig, error) {
	var row webhookRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, url, secret, events, enabled, created_at, updated_at
		 FROM webhook_configs WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return webhook.WebhookConfig{}, errx.NotFound("webhook not found")
		}
		return webhook.WebhookConfig{}, errx.Wrap(err, "failed to find webhook", errx.TypeInternal)
	}
	return row.toEntity(), nil
}

func (r *Repository) FindEnabledByEvent(ctx context.Context, event string) ([]webhook.WebhookConfig, error) {
	var rows []webhookRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, url, secret, events, enabled, created_at, updated_at
		 FROM webhook_configs WHERE enabled = true AND $1 = ANY(events)`, event)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find webhooks for event", errx.TypeInternal)
	}
	cfgs := make([]webhook.WebhookConfig, len(rows))
	for i, row := range rows {
		cfgs[i] = row.toEntity()
	}
	return cfgs, nil
}

func (r *Repository) List(ctx context.Context, page query.Pagination) (query.Paginated[webhook.WebhookConfig], error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM webhook_configs`); err != nil {
		return query.Paginated[webhook.WebhookConfig]{}, errx.Wrap(err, "failed to count webhooks", errx.TypeInternal)
	}

	var rows []webhookRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, url, secret, events, enabled, created_at, updated_at
		 FROM webhook_configs ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		page.Limit, page.Offset)
	if err != nil {
		return query.Paginated[webhook.WebhookConfig]{}, errx.Wrap(err, "failed to list webhooks", errx.TypeInternal)
	}

	items := make([]webhook.WebhookConfig, len(rows))
	for i, row := range rows {
		items[i] = row.toEntity()
	}

	return query.Paginated[webhook.WebhookConfig]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

func (r *Repository) Create(ctx context.Context, cfg webhook.WebhookConfig) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_configs (id, url, secret, events, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		cfg.ID, cfg.URL, cfg.Secret, pq.Array(cfg.Events), cfg.Enabled, cfg.CreatedAt, cfg.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "webhook")
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, cfg webhook.WebhookConfig) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE webhook_configs SET url = $2, events = $3, enabled = $4, updated_at = $5 WHERE id = $1`,
		cfg.ID, cfg.URL, pq.Array(cfg.Events), cfg.Enabled, cfg.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "webhook")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errx.NotFound("webhook not found")
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id identity.WebhookID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM webhook_configs WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete webhook", errx.TypeInternal)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errx.NotFound("webhook not found")
	}
	return nil
}

// ── Deliveries ──────────────────────────────────────────────────────

func (r *Repository) SaveDelivery(ctx context.Context, d webhook.WebhookDelivery) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (id, webhook_id, event_type, payload, status, status_code, attempts, last_error, next_retry_at, created_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		d.ID, d.WebhookID, d.EventType, d.Payload, d.Status, d.StatusCode, d.Attempts, d.LastError, d.NextRetryAt, d.CreatedAt, d.CompletedAt)
	if err != nil {
		return errx.Wrap(err, "failed to save webhook delivery", errx.TypeInternal)
	}
	return nil
}

func (r *Repository) UpdateDelivery(ctx context.Context, d webhook.WebhookDelivery) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE webhook_deliveries SET
		   status = $2, status_code = $3, attempts = $4, last_error = $5, next_retry_at = $6, completed_at = $7
		 WHERE id = $1`,
		d.ID, d.Status, d.StatusCode, d.Attempts, d.LastError, d.NextRetryAt, d.CompletedAt)
	if err != nil {
		return errx.Wrap(err, "failed to update webhook delivery", errx.TypeInternal)
	}
	return nil
}

func (r *Repository) FindPendingDeliveries(ctx context.Context, limit int) ([]webhook.WebhookDelivery, error) {
	var deliveries []webhook.WebhookDelivery
	err := r.db.SelectContext(ctx, &deliveries,
		`SELECT id, webhook_id, event_type, payload, status, status_code, attempts, last_error, next_retry_at, created_at, completed_at
		 FROM webhook_deliveries
		 WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		 ORDER BY created_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find pending deliveries", errx.TypeInternal)
	}
	if deliveries == nil {
		deliveries = []webhook.WebhookDelivery{}
	}
	return deliveries, nil
}

func (r *Repository) ListDeliveries(ctx context.Context, webhookID identity.WebhookID, page query.Pagination) (query.Paginated[webhook.WebhookDelivery], error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = $1`, webhookID); err != nil {
		return query.Paginated[webhook.WebhookDelivery]{}, errx.Wrap(err, "failed to count webhook deliveries", errx.TypeInternal)
	}

	var items []webhook.WebhookDelivery
	err := r.db.SelectContext(ctx, &items,
		`SELECT id, webhook_id, event_type, payload, status, status_code, attempts, last_error, next_retry_at, created_at, completed_at
		 FROM webhook_deliveries WHERE webhook_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		webhookID, page.Limit, page.Offset)
	if err != nil {
		return query.Paginated[webhook.WebhookDelivery]{}, errx.Wrap(err, "failed to list webhook deliveries", errx.TypeInternal)
	}
	if items == nil {
		items = []webhook.WebhookDelivery{}
	}

	return query.Paginated[webhook.WebhookDelivery]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────

func wrapPgError(err error, entity string) error {
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Code == "23505" {
			return errx.Conflict(entity + " already exists")
		}
	}
	return errx.Wrap(err, "failed to persist "+entity, errx.TypeInternal)
}
