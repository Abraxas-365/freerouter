package usageinfra

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/jmoiron/sqlx"
)

type PostgresUsageRepository struct {
	db *sqlx.DB
}

func NewPostgresUsageRepository(db *sqlx.DB) usage.UsageRepository {
	return &PostgresUsageRepository{db: db}
}

const logColumns = `
	id, tenant_id, api_key_id, provider_key_id,
	requested_model, used_model, used_provider, mapping_id,
	prompt_tokens, completion_tokens, total_tokens, cached_tokens,
	input_cost, output_cost, total_cost,
	duration_ms, streamed, status_code, finish_reason,
	has_error, error_message, created_at`

func (r *PostgresUsageRepository) Create(ctx context.Context, log usage.UsageLog) error {
	query := `
		INSERT INTO usage_logs (
			id, tenant_id, api_key_id, provider_key_id,
			requested_model, used_model, used_provider, mapping_id,
			prompt_tokens, completion_tokens, total_tokens, cached_tokens,
			input_cost, output_cost, total_cost,
			duration_ms, streamed, status_code, finish_reason,
			has_error, error_message, created_at
		) VALUES (
			:id, :tenant_id, :api_key_id, :provider_key_id,
			:requested_model, :used_model, :used_provider, :mapping_id,
			:prompt_tokens, :completion_tokens, :total_tokens, :cached_tokens,
			:input_cost, :output_cost, :total_cost,
			:duration_ms, :streamed, :status_code, :finish_reason,
			:has_error, :error_message, :created_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, log)
	if err != nil {
		return errx.Wrap(err, "failed to create usage log", errx.TypeInternal).
			WithDetail("log_id", log.ID.String())
	}
	return nil
}

func (r *PostgresUsageRepository) FindByID(ctx context.Context, id kernel.UsageLogID) (*usage.UsageLog, error) {
	query := `SELECT ` + logColumns + ` FROM usage_logs WHERE id = $1`

	var log usage.UsageLog
	err := r.db.GetContext(ctx, &log, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, usage.ErrLogNotFound().WithDetail("log_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to find usage log", errx.TypeInternal).
			WithDetail("log_id", id.String())
	}
	return &log, nil
}

func (r *PostgresUsageRepository) Query(ctx context.Context, q usage.UsageQuery) ([]*usage.UsageLog, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, q.TenantID.String())
	argIdx++

	if q.Model != "" {
		conditions = append(conditions, fmt.Sprintf("requested_model = $%d", argIdx))
		args = append(args, q.Model)
		argIdx++
	}
	if q.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("used_provider = $%d", argIdx))
		args = append(args, q.Provider)
		argIdx++
	}
	if q.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *q.From)
		argIdx++
	}
	if q.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *q.To)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count total
	countQuery := "SELECT COUNT(*) FROM usage_logs " + where
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, errx.Wrap(err, "failed to count usage logs", errx.TypeInternal)
	}

	// Fetch page
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf("SELECT %s FROM usage_logs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		logColumns, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var logs []usage.UsageLog
	if err := r.db.SelectContext(ctx, &logs, dataQuery, args...); err != nil {
		return nil, 0, errx.Wrap(err, "failed to query usage logs", errx.TypeInternal)
	}

	result := make([]*usage.UsageLog, len(logs))
	for i := range logs {
		result[i] = &logs[i]
	}
	return result, total, nil
}

func (r *PostgresUsageRepository) GetSummary(ctx context.Context, tenantID kernel.TenantID, from, to *time.Time) (*usage.UsageSummary, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, tenantID.String())
	argIdx++

	if from != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *to)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			COALESCE(COUNT(*), 0) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost,
			COALESCE(SUM(CASE WHEN has_error THEN 1 ELSE 0 END), 0) as error_count
		FROM usage_logs %s`, where)

	var summary usage.UsageSummary
	summary.TenantID = tenantID
	if err := r.db.GetContext(ctx, &summary, query, args...); err != nil {
		return nil, errx.Wrap(err, "failed to get usage summary", errx.TypeInternal)
	}
	return &summary, nil
}

func (r *PostgresUsageRepository) GetSummaryByModel(ctx context.Context, tenantID kernel.TenantID, from, to *time.Time) ([]usage.ModelUsageSummary, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, tenantID.String())
	argIdx++

	if from != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *from)
		argIdx++
	}
	if to != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *to)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			requested_model,
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(total_cost), 0) as total_cost
		FROM usage_logs %s
		GROUP BY requested_model
		ORDER BY total_cost DESC`, where)

	var summaries []usage.ModelUsageSummary
	if err := r.db.SelectContext(ctx, &summaries, query, args...); err != nil {
		return nil, errx.Wrap(err, "failed to get usage summary by model", errx.TypeInternal)
	}
	return summaries, nil
}
