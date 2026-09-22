package usagepg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/usage"
	"github.com/jmoiron/sqlx"
)

const columns = `id, key_id, requested_model, used_model, provider_id, mapping_id,
	prompt_tokens, completion_tokens, total_tokens, cached_tokens,
	input_cost, output_cost, total_cost,
	duration_ms, streamed, status_code, finish_reason,
	has_error, error_message, is_fallback, created_at`

type Repository struct{ db *sqlx.DB }

var _ usage.Repository = (*Repository)(nil)

func New(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, log usage.UsageLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO usage_logs (
			id, key_id, requested_model, used_model, provider_id, mapping_id,
			prompt_tokens, completion_tokens, total_tokens, cached_tokens,
			input_cost, output_cost, total_cost,
			duration_ms, streamed, status_code, finish_reason,
			has_error, error_message, is_fallback, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13,
			$14, $15, $16, $17,
			$18, $19, $20, $21
		)`,
		log.ID, log.KeyID, log.RequestedModel, log.UsedModel, log.ProviderID, log.MappingID,
		log.PromptTokens, log.CompletionTokens, log.TotalTokens, log.CachedTokens,
		log.InputCost, log.OutputCost, log.TotalCost,
		log.DurationMs, log.Streamed, log.StatusCode, log.FinishReason,
		log.HasError, log.ErrorMessage, log.IsFallback, log.CreatedAt)
	if err != nil {
		return errx.Wrap(err, "failed to create usage log", errx.TypeInternal)
	}
	return nil
}

func (r *Repository) Find(ctx context.Context, id identity.UsageLogID) (usage.UsageLog, error) {
	var log usage.UsageLog
	err := r.db.GetContext(ctx, &log,
		`SELECT `+columns+` FROM usage_logs WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return log, errx.NotFound("usage log not found")
		}
		return log, errx.Wrap(err, "failed to find usage log", errx.TypeInternal)
	}
	return log, nil
}

func (r *Repository) List(ctx context.Context, filter usage.Filter, page query.Pagination) (query.Paginated[usage.UsageLog], error) {
	var (
		where []string
		args  []interface{}
		idx   = 1
	)

	if filter.Model != nil && *filter.Model != "" {
		where = append(where, fmt.Sprintf("requested_model = $%d", idx))
		args = append(args, *filter.Model)
		idx++
	}
	if filter.Provider != nil && *filter.Provider != "" {
		where = append(where, fmt.Sprintf("provider_id = $%d", idx))
		args = append(args, *filter.Provider)
		idx++
	}
	if filter.HasError != nil {
		where = append(where, fmt.Sprintf("has_error = $%d", idx))
		args = append(args, *filter.HasError)
		idx++
	}
	if filter.From != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *filter.To)
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.GetContext(ctx, &total,
		fmt.Sprintf("SELECT COUNT(*) FROM usage_logs %s", whereClause), args...); err != nil {
		return query.Paginated[usage.UsageLog]{}, errx.Wrap(err, "failed to count usage logs", errx.TypeInternal)
	}

	dataQ := fmt.Sprintf(
		`SELECT `+columns+` FROM usage_logs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, idx, idx+1)
	args = append(args, page.Limit, page.Offset)

	var items []usage.UsageLog
	if err := r.db.SelectContext(ctx, &items, dataQ, args...); err != nil {
		return query.Paginated[usage.UsageLog]{}, errx.Wrap(err, "failed to list usage logs", errx.TypeInternal)
	}
	if items == nil {
		items = []usage.UsageLog{}
	}

	return query.Paginated[usage.UsageLog]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

func (r *Repository) GetSummary(ctx context.Context, from, to *time.Time) (*usage.Summary, error) {
	where, args := timeFilter(from, to)

	q := fmt.Sprintf(`
		SELECT
			COALESCE(COUNT(*), 0)                                       AS total_requests,
			COALESCE(SUM(total_tokens), 0)                              AS total_tokens,
			COALESCE(SUM(prompt_tokens), 0)                             AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0)                         AS completion_tokens,
			COALESCE(SUM(total_cost), 0)                                AS total_cost,
			COALESCE(SUM(CASE WHEN has_error THEN 1 ELSE 0 END), 0)    AS error_count
		FROM usage_logs %s`, where)

	var s usage.Summary
	if err := r.db.GetContext(ctx, &s, q, args...); err != nil {
		return nil, errx.Wrap(err, "failed to get usage summary", errx.TypeInternal)
	}
	return &s, nil
}

func (r *Repository) GetSummaryByModel(ctx context.Context, from, to *time.Time) ([]usage.ModelSummary, error) {
	where, args := timeFilter(from, to)

	q := fmt.Sprintf(`
		SELECT
			requested_model,
			COUNT(*)                            AS total_requests,
			COALESCE(SUM(total_tokens), 0)      AS total_tokens,
			COALESCE(SUM(prompt_tokens), 0)     AS prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
			COALESCE(SUM(total_cost), 0)        AS total_cost
		FROM usage_logs %s
		GROUP BY requested_model
		ORDER BY total_cost DESC`, where)

	var items []usage.ModelSummary
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, errx.Wrap(err, "failed to get usage summary by model", errx.TypeInternal)
	}
	if items == nil {
		items = []usage.ModelSummary{}
	}
	return items, nil
}

// timeFilter builds a WHERE clause for optional from/to time range.
func timeFilter(from, to *time.Time) (string, []interface{}) {
	var conds []string
	var args []interface{}
	idx := 1

	if from != nil {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *from)
		idx++
	}
	if to != nil {
		conds = append(conds, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *to)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// ── Retention config ───────────────────────────────────────────────

func (r *Repository) GetRetention(ctx context.Context) (usage.RetentionConfig, error) {
	var cfg usage.RetentionConfig
	err := r.db.GetContext(ctx, &cfg,
		`SELECT id, retention_days, retain_messages, retain_response_body, created_at, updated_at
		 FROM usage_retention_config ORDER BY created_at ASC LIMIT 1`)
	if err != nil {
		if err == sql.ErrNoRows {
			return cfg, errx.NotFound("retention config not found")
		}
		return cfg, errx.Wrap(err, "failed to get retention config", errx.TypeInternal)
	}
	return cfg, nil
}

func (r *Repository) UpsertRetention(ctx context.Context, cfg usage.RetentionConfig) error {
	// Global singleton: try update first, insert if no row exists.
	res, err := r.db.ExecContext(ctx,
		`UPDATE usage_retention_config
		 SET retention_days = $2, retain_messages = $3, retain_response_body = $4, updated_at = $5
		 WHERE id = $1`,
		cfg.ID, cfg.RetentionDays, cfg.RetainMessages, cfg.RetainResponseBody, cfg.UpdatedAt)
	if err != nil {
		return errx.Wrap(err, "failed to update retention config", errx.TypeInternal)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return nil
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO usage_retention_config
		 (id, retention_days, retain_messages, retain_response_body, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		cfg.ID, cfg.RetentionDays, cfg.RetainMessages, cfg.RetainResponseBody, cfg.CreatedAt, cfg.UpdatedAt)
	if err != nil {
		return errx.Wrap(err, "failed to create retention config", errx.TypeInternal)
	}
	return nil
}

func (r *Repository) DeleteRetention(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM usage_retention_config`); err != nil {
		return errx.Wrap(err, "failed to delete retention config", errx.TypeInternal)
	}
	return nil
}

// PurgeOlderThan deletes usage logs older than the given number of days and
// returns the number of rows removed.
func (r *Repository) PurgeOlderThan(ctx context.Context, days int) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM usage_logs WHERE created_at < now() - ($1 * interval '1 day')`, days)
	if err != nil {
		return 0, errx.Wrap(err, "failed to purge usage logs", errx.TypeInternal)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, errx.Wrap(err, "failed to get purge rows affected", errx.TypeInternal)
	}
	return n, nil
}
