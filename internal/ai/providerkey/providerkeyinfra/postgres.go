package providerkeyinfra

import (
	"context"
	"database/sql"

	"github.com/Abraxas-365/freerouter/internal/ai/providerkey"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/jmoiron/sqlx"
)

type PostgresProviderKeyRepository struct {
	db *sqlx.DB
}

func NewPostgresProviderKeyRepository(db *sqlx.DB) providerkey.ProviderKeyRepository {
	return &PostgresProviderKeyRepository{db: db}
}

const keyColumns = `
	id, provider_id, tenant_id,
	token_ciphertext, token_masked, token_hash, base_url,
	name, description, managed, status, sort_order,
	created_at, updated_at`

func (r *PostgresProviderKeyRepository) FindByID(ctx context.Context, id kernel.ProviderKeyID) (*providerkey.ProviderKey, error) {
	query := `SELECT ` + keyColumns + ` FROM provider_keys WHERE id = $1`

	var k providerkey.ProviderKey
	err := r.db.GetContext(ctx, &k, query, id.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, providerkey.ErrKeyNotFound().WithDetail("key_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to find provider key by id", errx.TypeInternal).
			WithDetail("key_id", id.String())
	}
	return &k, nil
}

func (r *PostgresProviderKeyRepository) FindByProvider(ctx context.Context, providerID kernel.ProviderID) ([]*providerkey.ProviderKey, error) {
	query := `SELECT ` + keyColumns + ` FROM provider_keys WHERE provider_id = $1 ORDER BY COALESCE(sort_order, 2147483647), created_at ASC`

	var keys []providerkey.ProviderKey
	err := r.db.SelectContext(ctx, &keys, query, providerID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find provider keys by provider", errx.TypeInternal).
			WithDetail("provider_id", providerID.String())
	}

	result := make([]*providerkey.ProviderKey, len(keys))
	for i := range keys {
		result[i] = &keys[i]
	}
	return result, nil
}

func (r *PostgresProviderKeyRepository) FindByTenant(ctx context.Context, tenantID kernel.TenantID) ([]*providerkey.ProviderKey, error) {
	query := `SELECT ` + keyColumns + ` FROM provider_keys WHERE tenant_id = $1 ORDER BY provider_id, COALESCE(sort_order, 2147483647), created_at ASC`

	var keys []providerkey.ProviderKey
	err := r.db.SelectContext(ctx, &keys, query, tenantID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find provider keys by tenant", errx.TypeInternal).
			WithDetail("tenant_id", tenantID.String())
	}

	result := make([]*providerkey.ProviderKey, len(keys))
	for i := range keys {
		result[i] = &keys[i]
	}
	return result, nil
}

func (r *PostgresProviderKeyRepository) FindManaged(ctx context.Context) ([]*providerkey.ProviderKey, error) {
	query := `SELECT ` + keyColumns + ` FROM provider_keys WHERE managed = true ORDER BY provider_id, COALESCE(sort_order, 2147483647), created_at ASC`

	var keys []providerkey.ProviderKey
	err := r.db.SelectContext(ctx, &keys, query)
	if err != nil {
		return nil, errx.Wrap(err, "failed to find managed provider keys", errx.TypeInternal)
	}

	result := make([]*providerkey.ProviderKey, len(keys))
	for i := range keys {
		result[i] = &keys[i]
	}
	return result, nil
}

func (r *PostgresProviderKeyRepository) FindActiveByProvider(ctx context.Context, providerID kernel.ProviderID) ([]*providerkey.ProviderKey, error) {
	query := `SELECT ` + keyColumns + ` FROM provider_keys WHERE provider_id = $1 AND status = 'active' ORDER BY COALESCE(sort_order, 2147483647), created_at ASC`

	var keys []providerkey.ProviderKey
	err := r.db.SelectContext(ctx, &keys, query, providerID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to find active provider keys", errx.TypeInternal).
			WithDetail("provider_id", providerID.String())
	}

	result := make([]*providerkey.ProviderKey, len(keys))
	for i := range keys {
		result[i] = &keys[i]
	}
	return result, nil
}

func (r *PostgresProviderKeyRepository) Save(ctx context.Context, k providerkey.ProviderKey) error {
	exists, err := r.exists(ctx, k.ID)
	if err != nil {
		return errx.Wrap(err, "failed to check provider key existence", errx.TypeInternal)
	}
	if exists {
		return r.update(ctx, k)
	}
	return r.create(ctx, k)
}

func (r *PostgresProviderKeyRepository) create(ctx context.Context, k providerkey.ProviderKey) error {
	query := `
		INSERT INTO provider_keys (
			id, provider_id, tenant_id,
			token_ciphertext, token_masked, token_hash, base_url,
			name, description, managed, status, sort_order,
			created_at, updated_at
		) VALUES (
			:id, :provider_id, :tenant_id,
			:token_ciphertext, :token_masked, :token_hash, :base_url,
			:name, :description, :managed, :status, :sort_order,
			:created_at, :updated_at
		)`

	_, err := r.db.NamedExecContext(ctx, query, k)
	if err != nil {
		return errx.Wrap(err, "failed to create provider key", errx.TypeInternal).
			WithDetail("key_id", k.ID.String())
	}
	return nil
}

func (r *PostgresProviderKeyRepository) update(ctx context.Context, k providerkey.ProviderKey) error {
	query := `
		UPDATE provider_keys SET
			provider_id = :provider_id, tenant_id = :tenant_id,
			token_ciphertext = :token_ciphertext, token_masked = :token_masked,
			token_hash = :token_hash, base_url = :base_url,
			name = :name, description = :description,
			managed = :managed, status = :status, sort_order = :sort_order,
			updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, k)
	if err != nil {
		return errx.Wrap(err, "failed to update provider key", errx.TypeInternal).
			WithDetail("key_id", k.ID.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return providerkey.ErrKeyNotFound().WithDetail("key_id", k.ID.String())
	}
	return nil
}

func (r *PostgresProviderKeyRepository) Delete(ctx context.Context, id kernel.ProviderKeyID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM provider_keys WHERE id = $1`, id.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete provider key", errx.TypeInternal).
			WithDetail("key_id", id.String())
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return providerkey.ErrKeyNotFound().WithDetail("key_id", id.String())
	}
	return nil
}

func (r *PostgresProviderKeyRepository) exists(ctx context.Context, id kernel.ProviderKeyID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM provider_keys WHERE id = $1)`, id.String())
	return exists, err
}
