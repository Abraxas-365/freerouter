package providerpg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/provider"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ════════════════════════════════════════════════════════════════════
// Provider Repository
// ════════════════════════════════════════════════════════════════════

type ProviderRepo struct{ db *sqlx.DB }

var _ provider.Repository = (*ProviderRepo)(nil)

func NewProvider(db *sqlx.DB) *ProviderRepo { return &ProviderRepo{db: db} }

func (r *ProviderRepo) Create(ctx context.Context, id identity.ProviderID, input provider.Create) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO providers (id, name, protocol, description, website, base_url, streaming)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, input.Name, input.Protocol, input.Description, input.Website, input.BaseURL, input.Streaming)
	if err != nil {
		return wrapPgError(err, "provider")
	}
	return nil
}

func (r *ProviderRepo) Find(ctx context.Context, id identity.ProviderID) (provider.Provider, error) {
	var p provider.Provider
	err := r.db.GetContext(ctx, &p,
		`SELECT id, name, protocol, description, website, base_url, status, streaming, created_at, updated_at
		 FROM providers WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return p, errx.NotFound("provider not found")
		}
		return p, errx.Wrap(err, "failed to find provider", errx.TypeInternal)
	}
	return p, nil
}

func (r *ProviderRepo) List(ctx context.Context, filter provider.Filter, page query.Pagination) (query.Paginated[provider.Provider], error) {
	var (
		where []string
		args  []interface{}
		idx   = 1
	)

	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}
	if filter.Search != nil && *filter.Search != "" {
		where = append(where, fmt.Sprintf("name ILIKE $%d", idx))
		args = append(args, "%"+*filter.Search+"%")
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT COUNT(*) FROM providers %s", whereClause), args...); err != nil {
		return query.Paginated[provider.Provider]{}, errx.Wrap(err, "failed to count providers", errx.TypeInternal)
	}

	dataQ := fmt.Sprintf(
		`SELECT id, name, protocol, description, website, base_url, status, streaming, created_at, updated_at
		 FROM providers %s ORDER BY name LIMIT $%d OFFSET $%d`,
		whereClause, idx, idx+1)
	args = append(args, page.Limit, page.Offset)

	var items []provider.Provider
	if err := r.db.SelectContext(ctx, &items, dataQ, args...); err != nil {
		return query.Paginated[provider.Provider]{}, errx.Wrap(err, "failed to list providers", errx.TypeInternal)
	}
	if items == nil {
		items = []provider.Provider{}
	}

	return query.Paginated[provider.Provider]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

func (r *ProviderRepo) Update(ctx context.Context, id identity.ProviderID, input provider.Update) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE providers SET
			name        = COALESCE($2, name),
			protocol    = COALESCE($3, protocol),
			description = COALESCE($4, description),
			website     = COALESCE($5, website),
			base_url    = COALESCE($6, base_url),
			status      = COALESCE($7, status),
			streaming   = COALESCE($8, streaming),
			updated_at  = NOW()
		 WHERE id = $1`,
		id, input.Name, input.Protocol, input.Description, input.Website, input.BaseURL, input.Status, input.Streaming)
	if err != nil {
		return wrapPgError(err, "provider")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("provider not found")
	}
	return nil
}

func (r *ProviderRepo) Delete(ctx context.Context, id identity.ProviderID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM providers WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete provider", errx.TypeInternal)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("provider not found")
	}
	return nil
}

// ════════════════════════════════════════════════════════════════════
// Model Repository
// ════════════════════════════════════════════════════════════════════

type ModelRepo struct{ db *sqlx.DB }

var _ provider.ModelRepository = (*ModelRepo)(nil)

func NewModel(db *sqlx.DB) *ModelRepo { return &ModelRepo{db: db} }

func (r *ModelRepo) Create(ctx context.Context, id identity.ModelID, input provider.CreateModel) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO models (id, name, description, family, free)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, input.Name, input.Description, input.Family, input.Free)
	if err != nil {
		return wrapPgError(err, "model")
	}
	return nil
}

func (r *ModelRepo) Find(ctx context.Context, id identity.ModelID) (provider.Model, error) {
	var m provider.Model
	err := r.db.GetContext(ctx, &m,
		`SELECT id, name, description, family, stability, status, free, released_at, created_at, updated_at
		 FROM models WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, errx.NotFound("model not found")
		}
		return m, errx.Wrap(err, "failed to find model", errx.TypeInternal)
	}
	return m, nil
}

func (r *ModelRepo) FindByName(ctx context.Context, name string) (provider.Model, error) {
	var m provider.Model
	err := r.db.GetContext(ctx, &m,
		`SELECT id, name, description, family, stability, status, free, released_at, created_at, updated_at
		 FROM models WHERE name = $1`, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, errx.NotFound("model not found")
		}
		return m, errx.Wrap(err, "failed to find model by name", errx.TypeInternal)
	}
	return m, nil
}

func (r *ModelRepo) List(ctx context.Context, filter provider.ModelFilter, page query.Pagination) (query.Paginated[provider.Model], error) {
	var (
		where []string
		args  []interface{}
		idx   = 1
	)

	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}
	if filter.Family != nil && *filter.Family != "" {
		where = append(where, fmt.Sprintf("family = $%d", idx))
		args = append(args, *filter.Family)
		idx++
	}
	if filter.Search != nil && *filter.Search != "" {
		where = append(where, fmt.Sprintf("name ILIKE $%d", idx))
		args = append(args, "%"+*filter.Search+"%")
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT COUNT(*) FROM models %s", whereClause), args...); err != nil {
		return query.Paginated[provider.Model]{}, errx.Wrap(err, "failed to count models", errx.TypeInternal)
	}

	dataQ := fmt.Sprintf(
		`SELECT id, name, description, family, stability, status, free, released_at, created_at, updated_at
		 FROM models %s ORDER BY name LIMIT $%d OFFSET $%d`,
		whereClause, idx, idx+1)
	args = append(args, page.Limit, page.Offset)

	var items []provider.Model
	if err := r.db.SelectContext(ctx, &items, dataQ, args...); err != nil {
		return query.Paginated[provider.Model]{}, errx.Wrap(err, "failed to list models", errx.TypeInternal)
	}
	if items == nil {
		items = []provider.Model{}
	}

	return query.Paginated[provider.Model]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

func (r *ModelRepo) Update(ctx context.Context, id identity.ModelID, input provider.UpdateModel) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE models SET
			name        = COALESCE($2, name),
			description = COALESCE($3, description),
			family      = COALESCE($4, family),
			stability   = COALESCE($5, stability),
			status      = COALESCE($6, status),
			free        = COALESCE($7, free),
			updated_at  = NOW()
		 WHERE id = $1`,
		id, input.Name, input.Description, input.Family, input.Stability, input.Status, input.Free)
	if err != nil {
		return wrapPgError(err, "model")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("model not found")
	}
	return nil
}

func (r *ModelRepo) Delete(ctx context.Context, id identity.ModelID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM models WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete model", errx.TypeInternal)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("model not found")
	}
	return nil
}

// ════════════════════════════════════════════════════════════════════
// Mapping Repository
// ════════════════════════════════════════════════════════════════════

type MappingRepo struct{ db *sqlx.DB }

var _ provider.MappingRepository = (*MappingRepo)(nil)

func NewMapping(db *sqlx.DB) *MappingRepo { return &MappingRepo{db: db} }

func (r *MappingRepo) Create(ctx context.Context, id identity.MappingID, input provider.CreateMapping) error {
	var region *string
	if input.Region != "" {
		region = &input.Region
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_provider_mappings (
			id, model_id, provider_id, external_id,
			input_price, output_price, cached_input_price, request_price, image_input_price,
			audio_price_per_minute, speech_price_per_1k_chars, rerank_price_per_1k,
			context_size, max_output,
			streaming, vision, reasoning, tools, json_output, audio, speech, moderation, rerank,
			region
		 ) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12,
			$13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22, $23,
			$24
		 )`,
		id, input.ModelID, input.ProviderID, input.ExternalID,
		input.InputPrice, input.OutputPrice, input.CachedInputPrice, input.RequestPrice, input.ImageInputPrice,
		input.AudioPricePerMinute, input.SpeechPricePer1kChars, input.RerankPricePer1k,
		input.ContextSize, input.MaxOutput,
		input.Streaming, input.Vision, input.Reasoning, input.Tools, input.JSONOutput, input.Audio, input.Speech, input.Moderation, input.Rerank,
		region)
	if err != nil {
		return wrapPgError(err, "mapping")
	}
	return nil
}

func (r *MappingRepo) Find(ctx context.Context, id identity.MappingID) (provider.ModelProviderMapping, error) {
	var m provider.ModelProviderMapping
	err := r.db.GetContext(ctx, &m, `SELECT * FROM model_provider_mappings WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, errx.NotFound("mapping not found")
		}
		return m, errx.Wrap(err, "failed to find mapping", errx.TypeInternal)
	}
	return m, nil
}

func (r *MappingRepo) List(ctx context.Context, filter provider.MappingFilter, page query.Pagination) (query.Paginated[provider.ModelProviderMapping], error) {
	var (
		where []string
		args  []interface{}
		idx   = 1
	)

	if filter.ModelID != nil {
		where = append(where, fmt.Sprintf("model_id = $%d", idx))
		args = append(args, *filter.ModelID)
		idx++
	}
	if filter.ProviderID != nil {
		where = append(where, fmt.Sprintf("provider_id = $%d", idx))
		args = append(args, *filter.ProviderID)
		idx++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT COUNT(*) FROM model_provider_mappings %s", whereClause), args...); err != nil {
		return query.Paginated[provider.ModelProviderMapping]{}, errx.Wrap(err, "failed to count mappings", errx.TypeInternal)
	}

	dataQ := fmt.Sprintf(`SELECT * FROM model_provider_mappings %s ORDER BY created_at LIMIT $%d OFFSET $%d`, whereClause, idx, idx+1)
	args = append(args, page.Limit, page.Offset)

	var items []provider.ModelProviderMapping
	if err := r.db.SelectContext(ctx, &items, dataQ, args...); err != nil {
		return query.Paginated[provider.ModelProviderMapping]{}, errx.Wrap(err, "failed to list mappings", errx.TypeInternal)
	}
	if items == nil {
		items = []provider.ModelProviderMapping{}
	}

	return query.Paginated[provider.ModelProviderMapping]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

func (r *MappingRepo) ListActiveByModel(ctx context.Context, model identity.ModelID) ([]provider.ModelProviderMapping, error) {
	var items []provider.ModelProviderMapping
	err := r.db.SelectContext(ctx, &items,
		`SELECT * FROM model_provider_mappings WHERE model_id = $1 AND status = 'active' ORDER BY created_at`, model)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list active mappings", errx.TypeInternal)
	}
	if items == nil {
		items = []provider.ModelProviderMapping{}
	}
	return items, nil
}

func (r *MappingRepo) Update(ctx context.Context, id identity.MappingID, input provider.UpdateMapping) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE model_provider_mappings SET
			external_id            = COALESCE($2, external_id),
			input_price            = COALESCE($3, input_price),
			output_price           = COALESCE($4, output_price),
			cached_input_price     = COALESCE($5, cached_input_price),
			request_price          = COALESCE($6, request_price),
			image_input_price      = COALESCE($7, image_input_price),
			audio_price_per_minute = COALESCE($8, audio_price_per_minute),
			speech_price_per_1k_chars = COALESCE($9, speech_price_per_1k_chars),
			rerank_price_per_1k    = COALESCE($10, rerank_price_per_1k),
			context_size           = COALESCE($11, context_size),
			max_output             = COALESCE($12, max_output),
			streaming              = COALESCE($13, streaming),
			vision                 = COALESCE($14, vision),
			reasoning              = COALESCE($15, reasoning),
			tools                  = COALESCE($16, tools),
			json_output            = COALESCE($17, json_output),
			audio                  = COALESCE($18, audio),
			speech                 = COALESCE($19, speech),
			moderation             = COALESCE($20, moderation),
			rerank                 = COALESCE($21, rerank),
			region                 = COALESCE($22, region),
			status                 = COALESCE($23, status),
			updated_at             = NOW()
		 WHERE id = $1`,
		id,
		input.ExternalID,
		input.InputPrice, input.OutputPrice, input.CachedInputPrice, input.RequestPrice, input.ImageInputPrice,
		input.AudioPricePerMinute, input.SpeechPricePer1kChars, input.RerankPricePer1k,
		input.ContextSize, input.MaxOutput,
		input.Streaming, input.Vision, input.Reasoning, input.Tools, input.JSONOutput, input.Audio, input.Speech, input.Moderation, input.Rerank,
		input.Region, input.Status)
	if err != nil {
		return wrapPgError(err, "mapping")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("mapping not found")
	}
	return nil
}

func (r *MappingRepo) Delete(ctx context.Context, id identity.MappingID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_provider_mappings WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete mapping", errx.TypeInternal)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("mapping not found")
	}
	return nil
}

// ════════════════════════════════════════════════════════════════════
// Fallback Repository
// ════════════════════════════════════════════════════════════════════

type FallbackRepo struct{ db *sqlx.DB }

var _ provider.FallbackRepository = (*FallbackRepo)(nil)

func NewFallback(db *sqlx.DB) *FallbackRepo { return &FallbackRepo{db: db} }

func (r *FallbackRepo) Create(ctx context.Context, id identity.ModelFallbackID, input provider.CreateFallback) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_fallbacks (id, model_id, fallback_model_id, priority)
		 VALUES ($1, $2, $3, $4)`,
		id, input.ModelID, input.FallbackModelID, input.Priority)
	if err != nil {
		return wrapPgError(err, "fallback")
	}
	return nil
}

func (r *FallbackRepo) FindByModel(ctx context.Context, model identity.ModelID) ([]provider.ModelFallback, error) {
	var items []provider.ModelFallback
	err := r.db.SelectContext(ctx, &items,
		`SELECT id, model_id, fallback_model_id, priority, enabled, created_at
		 FROM model_fallbacks WHERE model_id = $1 ORDER BY priority`, model)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list fallbacks", errx.TypeInternal)
	}
	if items == nil {
		items = []provider.ModelFallback{}
	}
	return items, nil
}

func (r *FallbackRepo) Delete(ctx context.Context, id identity.ModelFallbackID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_fallbacks WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete fallback", errx.TypeInternal)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("fallback not found")
	}
	return nil
}

// ════════════════════════════════════════════════════════════════════
// Helpers
// ════════════════════════════════════════════════════════════════════

func wrapPgError(err error, entity string) *errx.Error {
	var pqErr *pq.Error
	if errx.As(err, &pqErr) && pqErr.Code == "23505" {
		return errx.Conflict(entity + " already exists")
	}
	return errx.Wrap(err, "database error", errx.TypeInternal)
}
