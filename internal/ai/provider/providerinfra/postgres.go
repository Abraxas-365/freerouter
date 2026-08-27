package providerinfra

import (
	"context"
	"database/sql"

	"github.com/Abraxas-365/freerouter/internal/ai/provider"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/jmoiron/sqlx"
)

// ============================================================================
// Provider Repository
// ============================================================================

type PostgresProviderRepository struct {
	db *sqlx.DB
}

func NewPostgresProviderRepository(db *sqlx.DB) provider.ProviderRepository {
	return &PostgresProviderRepository{db: db}
}

func (r *PostgresProviderRepository) FindByID(ctx context.Context, id kernel.ProviderID) (*provider.Provider, error) {
	query := `
		SELECT id, name, description, website, status, streaming, created_at, updated_at
		FROM providers
		WHERE id = $1`

	var p provider.Provider
	err := r.db.GetContext(ctx, &p, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, provider.ErrProviderNotFound().WithDetail("provider_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to find provider by id", errx.TypeInternal).
			WithDetail("provider_id", id.String())
	}
	return &p, nil
}

func (r *PostgresProviderRepository) FindAll(ctx context.Context) ([]*provider.Provider, error) {
	query := `
		SELECT id, name, description, website, status, streaming, created_at, updated_at
		FROM providers
		ORDER BY name ASC`

	var providers []provider.Provider
	err := r.db.SelectContext(ctx, &providers, query)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find all providers", errx.TypeInternal)
	}

	result := make([]*provider.Provider, len(providers))
	for i := range providers {
		result[i] = &providers[i]
	}
	return result, nil
}

func (r *PostgresProviderRepository) FindActive(ctx context.Context) ([]*provider.Provider, error) {
	query := `
		SELECT id, name, description, website, status, streaming, created_at, updated_at
		FROM providers
		WHERE status = 'active'
		ORDER BY name ASC`

	var providers []provider.Provider
	err := r.db.SelectContext(ctx, &providers, query)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find active providers", errx.TypeInternal)
	}

	result := make([]*provider.Provider, len(providers))
	for i := range providers {
		result[i] = &providers[i]
	}
	return result, nil
}

func (r *PostgresProviderRepository) Save(ctx context.Context, p provider.Provider) error {
	exists, err := r.exists(ctx, p.ID)
	if err != nil {
		return errx.Wrap(err, "failed to check provider existence", errx.TypeInternal)
	}
	if exists {
		return r.update(ctx, p)
	}
	return r.create(ctx, p)
}

func (r *PostgresProviderRepository) create(ctx context.Context, p provider.Provider) error {
	query := `
		INSERT INTO providers (id, name, description, website, status, streaming, created_at, updated_at)
		VALUES (:id, :name, :description, :website, :status, :streaming, :created_at, :updated_at)`

	_, err := r.db.NamedExecContext(ctx, query, p)
	if err != nil {
		return errx.Wrap(err, "failed to create provider", errx.TypeInternal).
			WithDetail("provider_id", p.ID.String())
	}
	return nil
}

func (r *PostgresProviderRepository) update(ctx context.Context, p provider.Provider) error {
	query := `
		UPDATE providers SET
			name = :name, description = :description, website = :website,
			status = :status, streaming = :streaming, updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, p)
	if err != nil {
		return errx.Wrap(err, "failed to update provider", errx.TypeInternal).
			WithDetail("provider_id", p.ID.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return provider.ErrProviderNotFound().WithDetail("provider_id", p.ID.String())
	}
	return nil
}

func (r *PostgresProviderRepository) Delete(ctx context.Context, id kernel.ProviderID) error {
	query := `DELETE FROM providers WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete provider", errx.TypeInternal).
			WithDetail("provider_id", id.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return provider.ErrProviderNotFound().WithDetail("provider_id", id.String())
	}
	return nil
}

func (r *PostgresProviderRepository) exists(ctx context.Context, id kernel.ProviderID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM providers WHERE id = $1)`, id.String())
	return exists, err
}

// ============================================================================
// Model Repository
// ============================================================================

type PostgresModelRepository struct {
	db *sqlx.DB
}

func NewPostgresModelRepository(db *sqlx.DB) provider.ModelRepository {
	return &PostgresModelRepository{db: db}
}

const modelColumns = `id, name, description, family, stability, status, free, released_at, created_at, updated_at`

func (r *PostgresModelRepository) FindByID(ctx context.Context, id kernel.ModelID) (*provider.Model, error) {
	query := `SELECT ` + modelColumns + ` FROM models WHERE id = $1`

	var m provider.Model
	err := r.db.GetContext(ctx, &m, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, provider.ErrModelNotFound().WithDetail("model_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to find model by id", errx.TypeInternal).
			WithDetail("model_id", id.String())
	}
	return &m, nil
}

func (r *PostgresModelRepository) FindAll(ctx context.Context) ([]*provider.Model, error) {
	query := `SELECT ` + modelColumns + ` FROM models ORDER BY family, name ASC`

	var models []provider.Model
	err := r.db.SelectContext(ctx, &models, query)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find all models", errx.TypeInternal)
	}

	result := make([]*provider.Model, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *PostgresModelRepository) FindActive(ctx context.Context) ([]*provider.Model, error) {
	query := `SELECT ` + modelColumns + ` FROM models WHERE status = 'active' ORDER BY family, name ASC`

	var models []provider.Model
	err := r.db.SelectContext(ctx, &models, query)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find active models", errx.TypeInternal)
	}

	result := make([]*provider.Model, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *PostgresModelRepository) FindByFamily(ctx context.Context, family string) ([]*provider.Model, error) {
	query := `SELECT ` + modelColumns + ` FROM models WHERE family = $1 ORDER BY name ASC`

	var models []provider.Model
	err := r.db.SelectContext(ctx, &models, query, family)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find models by family", errx.TypeInternal).
			WithDetail("family", family)
	}

	result := make([]*provider.Model, len(models))
	for i := range models {
		result[i] = &models[i]
	}
	return result, nil
}

func (r *PostgresModelRepository) Save(ctx context.Context, m provider.Model) error {
	exists, err := r.exists(ctx, m.ID)
	if err != nil {
		return errx.Wrap(err, "failed to check model existence", errx.TypeInternal)
	}
	if exists {
		return r.update(ctx, m)
	}
	return r.create(ctx, m)
}

func (r *PostgresModelRepository) create(ctx context.Context, m provider.Model) error {
	query := `
		INSERT INTO models (id, name, description, family, stability, status, free, released_at, created_at, updated_at)
		VALUES (:id, :name, :description, :family, :stability, :status, :free, :released_at, :created_at, :updated_at)`

	_, err := r.db.NamedExecContext(ctx, query, m)
	if err != nil {
		return errx.Wrap(err, "failed to create model", errx.TypeInternal).
			WithDetail("model_id", m.ID.String())
	}
	return nil
}

func (r *PostgresModelRepository) update(ctx context.Context, m provider.Model) error {
	query := `
		UPDATE models SET
			name = :name, description = :description, family = :family,
			stability = :stability, status = :status, free = :free,
			released_at = :released_at, updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, m)
	if err != nil {
		return errx.Wrap(err, "failed to update model", errx.TypeInternal).
			WithDetail("model_id", m.ID.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return provider.ErrModelNotFound().WithDetail("model_id", m.ID.String())
	}
	return nil
}

func (r *PostgresModelRepository) Delete(ctx context.Context, id kernel.ModelID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM models WHERE id = $1`, id.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete model", errx.TypeInternal).
			WithDetail("model_id", id.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return provider.ErrModelNotFound().WithDetail("model_id", id.String())
	}
	return nil
}

func (r *PostgresModelRepository) exists(ctx context.Context, id kernel.ModelID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM models WHERE id = $1)`, id.String())
	return exists, err
}

// ============================================================================
// Mapping Repository
// ============================================================================

type PostgresMappingRepository struct {
	db *sqlx.DB
}

func NewPostgresMappingRepository(db *sqlx.DB) provider.MappingRepository {
	return &PostgresMappingRepository{db: db}
}

const mappingColumns = `
	id, model_id, provider_id, external_id,
	input_price, output_price, cached_input_price, request_price, image_input_price,
	context_size, max_output,
	streaming, vision, reasoning, tools, json_output,
	region, stability, status, created_at, updated_at`

func (r *PostgresMappingRepository) FindByID(ctx context.Context, id kernel.MappingID) (*provider.ModelProviderMapping, error) {
	query := `SELECT ` + mappingColumns + ` FROM model_provider_mappings WHERE id = $1`

	var m provider.ModelProviderMapping
	err := r.db.GetContext(ctx, &m, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, provider.ErrMappingNotFound().WithDetail("mapping_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to find mapping by id", errx.TypeInternal).
			WithDetail("mapping_id", id.String())
	}
	return &m, nil
}

func (r *PostgresMappingRepository) FindByModel(ctx context.Context, modelID kernel.ModelID) ([]*provider.ModelProviderMapping, error) {
	query := `SELECT ` + mappingColumns + ` FROM model_provider_mappings WHERE model_id = $1 ORDER BY provider_id ASC`

	var mappings []provider.ModelProviderMapping
	err := r.db.SelectContext(ctx, &mappings, query, modelID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find mappings by model", errx.TypeInternal).
			WithDetail("model_id", modelID.String())
	}

	result := make([]*provider.ModelProviderMapping, len(mappings))
	for i := range mappings {
		result[i] = &mappings[i]
	}
	return result, nil
}

func (r *PostgresMappingRepository) FindByProvider(ctx context.Context, providerID kernel.ProviderID) ([]*provider.ModelProviderMapping, error) {
	query := `SELECT ` + mappingColumns + ` FROM model_provider_mappings WHERE provider_id = $1 ORDER BY model_id ASC`

	var mappings []provider.ModelProviderMapping
	err := r.db.SelectContext(ctx, &mappings, query, providerID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find mappings by provider", errx.TypeInternal).
			WithDetail("provider_id", providerID.String())
	}

	result := make([]*provider.ModelProviderMapping, len(mappings))
	for i := range mappings {
		result[i] = &mappings[i]
	}
	return result, nil
}

func (r *PostgresMappingRepository) FindActiveByModel(ctx context.Context, modelID kernel.ModelID) ([]*provider.ModelProviderMapping, error) {
	query := `SELECT ` + mappingColumns + `
		FROM model_provider_mappings
		WHERE model_id = $1 AND status = 'active'
		ORDER BY provider_id ASC`

	var mappings []provider.ModelProviderMapping
	err := r.db.SelectContext(ctx, &mappings, query, modelID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find active mappings by model", errx.TypeInternal).
			WithDetail("model_id", modelID.String())
	}

	result := make([]*provider.ModelProviderMapping, len(mappings))
	for i := range mappings {
		result[i] = &mappings[i]
	}
	return result, nil
}

func (r *PostgresMappingRepository) Save(ctx context.Context, m provider.ModelProviderMapping) error {
	exists, err := r.exists(ctx, m.ID)
	if err != nil {
		return errx.Wrap(err, "failed to check mapping existence", errx.TypeInternal)
	}
	if exists {
		return r.update(ctx, m)
	}
	return r.create(ctx, m)
}

func (r *PostgresMappingRepository) create(ctx context.Context, m provider.ModelProviderMapping) error {
	query := `
		INSERT INTO model_provider_mappings (
			id, model_id, provider_id, external_id,
			input_price, output_price, cached_input_price, request_price, image_input_price,
			context_size, max_output,
			streaming, vision, reasoning, tools, json_output,
			region, stability, status, created_at, updated_at
		) VALUES (
			:id, :model_id, :provider_id, :external_id,
			:input_price, :output_price, :cached_input_price, :request_price, :image_input_price,
			:context_size, :max_output,
			:streaming, :vision, :reasoning, :tools, :json_output,
			:region, :stability, :status, :created_at, :updated_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, m)
	if err != nil {
		return errx.Wrap(err, "failed to create mapping", errx.TypeInternal).
			WithDetail("mapping_id", m.ID.String())
	}
	return nil
}

func (r *PostgresMappingRepository) update(ctx context.Context, m provider.ModelProviderMapping) error {
	query := `
		UPDATE model_provider_mappings SET
			model_id = :model_id, provider_id = :provider_id, external_id = :external_id,
			input_price = :input_price, output_price = :output_price,
			cached_input_price = :cached_input_price, request_price = :request_price,
			image_input_price = :image_input_price,
			context_size = :context_size, max_output = :max_output,
			streaming = :streaming, vision = :vision, reasoning = :reasoning,
			tools = :tools, json_output = :json_output,
			region = :region, stability = :stability, status = :status,
			updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, m)
	if err != nil {
		return errx.Wrap(err, "failed to update mapping", errx.TypeInternal).
			WithDetail("mapping_id", m.ID.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return provider.ErrMappingNotFound().WithDetail("mapping_id", m.ID.String())
	}
	return nil
}

func (r *PostgresMappingRepository) Delete(ctx context.Context, id kernel.MappingID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_provider_mappings WHERE id = $1`, id.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete mapping", errx.TypeInternal).
			WithDetail("mapping_id", id.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return provider.ErrMappingNotFound().WithDetail("mapping_id", id.String())
	}
	return nil
}

func (r *PostgresMappingRepository) exists(ctx context.Context, id kernel.MappingID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM model_provider_mappings WHERE id = $1)`, id.String())
	return exists, err
}
