package providerkeypg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/providerkey"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const columns = `id, provider_id, key_type, token_ciphertext, token_masked, token_hash,
	base_url, name, description, status, sort_order, created_at, updated_at`

type Repository struct{ db *sqlx.DB }

var _ providerkey.Repository = (*Repository)(nil)

func New(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, k providerkey.ProviderKey) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO provider_keys (
			id, provider_id, key_type, token_ciphertext, token_masked, token_hash,
			base_url, name, description, status, sort_order
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		k.ID, k.ProviderID, k.KeyType, k.TokenCiphertext, k.TokenMasked, k.TokenHash,
		k.BaseURL, k.Name, k.Description, k.Status, k.SortOrder)
	if err != nil {
		return wrapPgError(err)
	}
	return nil
}

func (r *Repository) Find(ctx context.Context, id identity.ProviderKeyID) (providerkey.ProviderKey, error) {
	var k providerkey.ProviderKey
	err := r.db.GetContext(ctx, &k,
		`SELECT `+columns+` FROM provider_keys WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return k, errx.NotFound("provider key not found")
		}
		return k, errx.Wrap(err, "failed to find provider key", errx.TypeInternal)
	}
	return k, nil
}

func (r *Repository) List(ctx context.Context, filter providerkey.Filter, page query.Pagination) (query.Paginated[providerkey.ProviderKey], error) {
	var (
		where []string
		args  []interface{}
		idx   = 1
	)

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
	if filter.KeyType != nil {
		where = append(where, fmt.Sprintf("key_type = $%d", idx))
		args = append(args, string(*filter.KeyType))
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.GetContext(ctx, &total, fmt.Sprintf("SELECT COUNT(*) FROM provider_keys %s", whereClause), args...); err != nil {
		return query.Paginated[providerkey.ProviderKey]{}, errx.Wrap(err, "failed to count provider keys", errx.TypeInternal)
	}

	dataQ := fmt.Sprintf(
		`SELECT `+columns+` FROM provider_keys %s ORDER BY COALESCE(sort_order, 2147483647), created_at LIMIT $%d OFFSET $%d`,
		whereClause, idx, idx+1)
	args = append(args, page.Limit, page.Offset)

	var items []providerkey.ProviderKey
	if err := r.db.SelectContext(ctx, &items, dataQ, args...); err != nil {
		return query.Paginated[providerkey.ProviderKey]{}, errx.Wrap(err, "failed to list provider keys", errx.TypeInternal)
	}
	if items == nil {
		items = []providerkey.ProviderKey{}
	}

	return query.Paginated[providerkey.ProviderKey]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

func (r *Repository) ListActiveByProvider(ctx context.Context, provider identity.ProviderID) ([]providerkey.ProviderKey, error) {
	var items []providerkey.ProviderKey
	err := r.db.SelectContext(ctx, &items,
		`SELECT `+columns+` FROM provider_keys WHERE provider_id = $1 AND status = 'active'
		 ORDER BY COALESCE(sort_order, 2147483647), created_at`, provider)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list active provider keys", errx.TypeInternal)
	}
	if items == nil {
		items = []providerkey.ProviderKey{}
	}
	return items, nil
}

func (r *Repository) ListOAuthKeys(ctx context.Context) ([]providerkey.ProviderKey, error) {
	var items []providerkey.ProviderKey
	err := r.db.SelectContext(ctx, &items,
		`SELECT `+columns+` FROM provider_keys WHERE key_type = 'oauth' AND status = 'active'
		 ORDER BY created_at`)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list oauth keys", errx.TypeInternal)
	}
	if items == nil {
		items = []providerkey.ProviderKey{}
	}
	return items, nil
}

func (r *Repository) Update(ctx context.Context, k providerkey.ProviderKey) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE provider_keys SET
			key_type         = $2,
			token_ciphertext = $3,
			token_masked     = $4,
			token_hash       = $5,
			base_url         = $6,
			name             = $7,
			description      = $8,
			status           = $9,
			sort_order       = $10,
			updated_at       = NOW()
		 WHERE id = $1`,
		k.ID, k.KeyType, k.TokenCiphertext, k.TokenMasked, k.TokenHash,
		k.BaseURL, k.Name, k.Description, k.Status, k.SortOrder)
	if err != nil {
		return wrapPgError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("provider key not found")
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id identity.ProviderKeyID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM provider_keys WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete provider key", errx.TypeInternal)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errx.NotFound("provider key not found")
	}
	return nil
}

func wrapPgError(err error) *errx.Error {
	var pqErr *pq.Error
	if errx.As(err, &pqErr) && pqErr.Code == "23505" {
		return errx.Conflict("provider key already exists")
	}
	return errx.Wrap(err, "database error", errx.TypeInternal)
}
