package usageinfra

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
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
	has_error, error_message,
	messages, response_body, raw_request, raw_response,
	upstream_request, upstream_response, is_debug,
	created_at`

// usageLogPersistence mirrors usage.UsageLog but represents JSONB content
// columns with sqlx/types.NullJSONText so they round-trip correctly through
// lib/pq (a plain json.RawMessage / []byte is otherwise sent as bytea).
type usageLogPersistence struct {
	ID       kernel.UsageLogID     `db:"id"`
	TenantID kernel.TenantID       `db:"tenant_id"`
	APIKeyID *kernel.APIKeyID      `db:"api_key_id"`
	KeyID    *kernel.ProviderKeyID `db:"provider_key_id"`

	RequestedModel string            `db:"requested_model"`
	UsedModel      string            `db:"used_model"`
	UsedProvider   kernel.ProviderID `db:"used_provider"`
	MappingID      kernel.MappingID  `db:"mapping_id"`

	PromptTokens     int `db:"prompt_tokens"`
	CompletionTokens int `db:"completion_tokens"`
	TotalTokens      int `db:"total_tokens"`
	CachedTokens     int `db:"cached_tokens"`

	InputCost  float64 `db:"input_cost"`
	OutputCost float64 `db:"output_cost"`
	TotalCost  float64 `db:"total_cost"`

	DurationMs   int    `db:"duration_ms"`
	Streamed     bool   `db:"streamed"`
	StatusCode   int    `db:"status_code"`
	FinishReason string `db:"finish_reason"`
	HasError     bool   `db:"has_error"`
	ErrorMessage string `db:"error_message"`

	Messages         types.NullJSONText `db:"messages"`
	ResponseBody     types.NullJSONText `db:"response_body"`
	RawRequest       types.NullJSONText `db:"raw_request"`
	RawResponse      types.NullJSONText `db:"raw_response"`
	UpstreamRequest  types.NullJSONText `db:"upstream_request"`
	UpstreamResponse types.NullJSONText `db:"upstream_response"`
	IsDebug          bool               `db:"is_debug"`

	CreatedAt time.Time `db:"created_at"`
}

func toNullJSONText(raw json.RawMessage) types.NullJSONText {
	if len(raw) == 0 {
		return types.NullJSONText{}
	}
	return types.NullJSONText{JSONText: types.JSONText(raw), Valid: true}
}

func fromNullJSONText(t types.NullJSONText) json.RawMessage {
	if !t.Valid {
		return nil
	}
	return json.RawMessage(t.JSONText)
}

func toPersistence(log usage.UsageLog) usageLogPersistence {
	return usageLogPersistence{
		ID:               log.ID,
		TenantID:         log.TenantID,
		APIKeyID:         log.APIKeyID,
		KeyID:            log.KeyID,
		RequestedModel:   log.RequestedModel,
		UsedModel:        log.UsedModel,
		UsedProvider:     log.UsedProvider,
		MappingID:        log.MappingID,
		PromptTokens:     log.PromptTokens,
		CompletionTokens: log.CompletionTokens,
		TotalTokens:      log.TotalTokens,
		CachedTokens:     log.CachedTokens,
		InputCost:        log.InputCost,
		OutputCost:       log.OutputCost,
		TotalCost:        log.TotalCost,
		DurationMs:       log.DurationMs,
		Streamed:         log.Streamed,
		StatusCode:       log.StatusCode,
		FinishReason:     log.FinishReason,
		HasError:         log.HasError,
		ErrorMessage:     log.ErrorMessage,
		Messages:         toNullJSONText(log.Messages),
		ResponseBody:     toNullJSONText(log.ResponseBody),
		RawRequest:       toNullJSONText(log.RawRequest),
		RawResponse:      toNullJSONText(log.RawResponse),
		UpstreamRequest:  toNullJSONText(log.UpstreamRequest),
		UpstreamResponse: toNullJSONText(log.UpstreamResponse),
		IsDebug:          log.IsDebug,
		CreatedAt:        log.CreatedAt,
	}
}

func (p usageLogPersistence) toDomain() usage.UsageLog {
	return usage.UsageLog{
		ID:               p.ID,
		TenantID:         p.TenantID,
		APIKeyID:         p.APIKeyID,
		KeyID:            p.KeyID,
		RequestedModel:   p.RequestedModel,
		UsedModel:        p.UsedModel,
		UsedProvider:     p.UsedProvider,
		MappingID:        p.MappingID,
		PromptTokens:     p.PromptTokens,
		CompletionTokens: p.CompletionTokens,
		TotalTokens:      p.TotalTokens,
		CachedTokens:     p.CachedTokens,
		InputCost:        p.InputCost,
		OutputCost:       p.OutputCost,
		TotalCost:        p.TotalCost,
		DurationMs:       p.DurationMs,
		Streamed:         p.Streamed,
		StatusCode:       p.StatusCode,
		FinishReason:     p.FinishReason,
		HasError:         p.HasError,
		ErrorMessage:     p.ErrorMessage,
		Messages:         fromNullJSONText(p.Messages),
		ResponseBody:     fromNullJSONText(p.ResponseBody),
		RawRequest:       fromNullJSONText(p.RawRequest),
		RawResponse:      fromNullJSONText(p.RawResponse),
		UpstreamRequest:  fromNullJSONText(p.UpstreamRequest),
		UpstreamResponse: fromNullJSONText(p.UpstreamResponse),
		IsDebug:          p.IsDebug,
		CreatedAt:        p.CreatedAt,
	}
}

func (r *PostgresUsageRepository) Create(ctx context.Context, log usage.UsageLog) error {
	query := `
		INSERT INTO usage_logs (
			id, tenant_id, api_key_id, provider_key_id,
			requested_model, used_model, used_provider, mapping_id,
			prompt_tokens, completion_tokens, total_tokens, cached_tokens,
			input_cost, output_cost, total_cost,
			duration_ms, streamed, status_code, finish_reason,
			has_error, error_message,
			messages, response_body, raw_request, raw_response,
			upstream_request, upstream_response, is_debug, created_at
		) VALUES (
			:id, :tenant_id, :api_key_id, :provider_key_id,
			:requested_model, :used_model, :used_provider, :mapping_id,
			:prompt_tokens, :completion_tokens, :total_tokens, :cached_tokens,
			:input_cost, :output_cost, :total_cost,
			:duration_ms, :streamed, :status_code, :finish_reason,
			:has_error, :error_message,
			:messages, :response_body, :raw_request, :raw_response,
			:upstream_request, :upstream_response, :is_debug, :created_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, toPersistence(log))
	if err != nil {
		return errx.Wrap(err, "failed to create usage log", errx.TypeInternal).
			WithDetail("log_id", log.ID.String())
	}
	return nil
}

func (r *PostgresUsageRepository) FindByID(ctx context.Context, id kernel.UsageLogID) (*usage.UsageLog, error) {
	query := `SELECT ` + logColumns + ` FROM usage_logs WHERE id = $1`

	var p usageLogPersistence
	err := r.db.GetContext(ctx, &p, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, usage.ErrLogNotFound().WithDetail("log_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to find usage log", errx.TypeInternal).
			WithDetail("log_id", id.String())
	}
	log := p.toDomain()
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

	var logs []usageLogPersistence
	if err := r.db.SelectContext(ctx, &logs, dataQuery, args...); err != nil {
		return nil, 0, errx.Wrap(err, "failed to query usage logs", errx.TypeInternal)
	}

	result := make([]*usage.UsageLog, len(logs))
	for i := range logs {
		log := logs[i].toDomain()
		result[i] = &log
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

// NullifyExpiredContent clears content payloads for logs older than the given time.
func (r *PostgresUsageRepository) NullifyExpiredContent(ctx context.Context, before time.Time) (int, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE usage_logs
		SET messages = NULL,
			response_body = NULL,
			raw_request = NULL,
			raw_response = NULL,
			upstream_request = NULL,
			upstream_response = NULL
		WHERE created_at < $1
		  AND (messages IS NOT NULL OR response_body IS NOT NULL OR raw_request IS NOT NULL)
	`, before)
	if err != nil {
		return 0, errx.Wrap(err, "failed to nullify expired content", errx.TypeInternal)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// ============================================================================
// Data Retention Config Repository
// ============================================================================

type PostgresDataRetentionRepository struct {
	db *sqlx.DB
}

func NewPostgresDataRetentionRepository(db *sqlx.DB) usage.DataRetentionRepository {
	return &PostgresDataRetentionRepository{db: db}
}

const retentionColumns = `
	tenant_id, retention_days, retain_messages, retain_response_body,
	retain_debug_payloads, created_at, updated_at`

func (r *PostgresDataRetentionRepository) GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*usage.DataRetentionConfig, error) {
	query := `SELECT ` + retentionColumns + ` FROM data_retention_configs WHERE tenant_id = $1`

	var cfg usage.DataRetentionConfig
	err := r.db.GetContext(ctx, &cfg, query, tenantID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errx.Wrap(err, "failed to get data retention config", errx.TypeInternal).
			WithDetail("tenant_id", tenantID.String())
	}
	return &cfg, nil
}

func (r *PostgresDataRetentionRepository) Upsert(ctx context.Context, cfg usage.DataRetentionConfig) (*usage.DataRetentionConfig, error) {
	query := `
		INSERT INTO data_retention_configs (
			tenant_id, retention_days, retain_messages, retain_response_body,
			retain_debug_payloads, created_at, updated_at
		) VALUES (
			:tenant_id, :retention_days, :retain_messages, :retain_response_body,
			:retain_debug_payloads, NOW(), NOW()
		)
		ON CONFLICT (tenant_id) DO UPDATE SET
			retention_days = EXCLUDED.retention_days,
			retain_messages = EXCLUDED.retain_messages,
			retain_response_body = EXCLUDED.retain_response_body,
			retain_debug_payloads = EXCLUDED.retain_debug_payloads,
			updated_at = NOW()
		RETURNING ` + retentionColumns

	rows, err := r.db.NamedQueryContext(ctx, query, cfg)
	if err != nil {
		return nil, errx.Wrap(err, "failed to upsert data retention config", errx.TypeInternal).
			WithDetail("tenant_id", cfg.TenantID.String())
	}
	defer rows.Close()

	var saved usage.DataRetentionConfig
	if rows.Next() {
		if err := rows.StructScan(&saved); err != nil {
			return nil, errx.Wrap(err, "failed to scan data retention config", errx.TypeInternal)
		}
	}
	return &saved, nil
}

func (r *PostgresDataRetentionRepository) Delete(ctx context.Context, tenantID kernel.TenantID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM data_retention_configs WHERE tenant_id = $1`, tenantID.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete data retention config", errx.TypeInternal).
			WithDetail("tenant_id", tenantID.String())
	}
	return nil
}

func (r *PostgresDataRetentionRepository) ListAll(ctx context.Context) ([]*usage.DataRetentionConfig, error) {
	query := `SELECT ` + retentionColumns + ` FROM data_retention_configs`

	var configs []usage.DataRetentionConfig
	if err := r.db.SelectContext(ctx, &configs, query); err != nil {
		return nil, errx.Wrap(err, "failed to list data retention configs", errx.TypeInternal)
	}

	result := make([]*usage.DataRetentionConfig, len(configs))
	for i := range configs {
		result[i] = &configs[i]
	}
	return result, nil
}
