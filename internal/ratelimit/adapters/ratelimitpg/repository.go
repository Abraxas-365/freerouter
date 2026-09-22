package ratelimitpg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/ratelimit"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository implements ratelimit.Repository with PostgreSQL.
type Repository struct{ db *sqlx.DB }

var _ ratelimit.Repository = (*Repository)(nil)

// New creates a rate limit config repository.
func New(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Create(ctx context.Context, cfg ratelimit.RateLimitConfig) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO rate_limit_configs (id, name, subject_id, rpm, max_concurrent, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		cfg.ID, cfg.Name, cfg.SubjectID, cfg.RPM, cfg.MaxConcurrent, cfg.CreatedAt, cfg.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "rate limit config")
	}
	return nil
}

func (r *Repository) Find(ctx context.Context, id identity.RateLimitConfigID) (ratelimit.RateLimitConfig, error) {
	var cfg ratelimit.RateLimitConfig
	err := r.db.GetContext(ctx, &cfg,
		`SELECT id, name, subject_id, rpm, max_concurrent, created_at, updated_at
		 FROM rate_limit_configs WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return cfg, errx.NotFound("rate limit config not found")
		}
		return cfg, errx.Wrap(err, "failed to find rate limit config", errx.TypeInternal)
	}
	return cfg, nil
}

func (r *Repository) FindBySubject(ctx context.Context, subjectID string) (ratelimit.RateLimitConfig, error) {
	var cfg ratelimit.RateLimitConfig
	err := r.db.GetContext(ctx, &cfg,
		`SELECT id, name, subject_id, rpm, max_concurrent, created_at, updated_at
		 FROM rate_limit_configs WHERE subject_id = $1`, subjectID)
	if err != nil {
		if err == sql.ErrNoRows {
			return cfg, errx.NotFound("rate limit config not found for subject")
		}
		return cfg, errx.Wrap(err, "failed to find rate limit config by subject", errx.TypeInternal)
	}
	return cfg, nil
}

func (r *Repository) Update(ctx context.Context, cfg ratelimit.RateLimitConfig) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE rate_limit_configs
		 SET name = $2, rpm = $3, max_concurrent = $4, updated_at = $5
		 WHERE id = $1`,
		cfg.ID, cfg.Name, cfg.RPM, cfg.MaxConcurrent, cfg.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "rate limit config")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errx.NotFound("rate limit config not found")
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id identity.RateLimitConfigID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM rate_limit_configs WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete rate limit config", errx.TypeInternal)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errx.NotFound("rate limit config not found")
	}
	return nil
}

func (r *Repository) List(ctx context.Context, filter ratelimit.Filter, page query.Pagination) (query.Paginated[ratelimit.RateLimitConfig], error) {
	var (
		where []string
		args  []interface{}
		idx   = 1
	)

	if filter.SubjectID != nil && *filter.SubjectID != "" {
		where = append(where, fmt.Sprintf("subject_id = $%d", idx))
		args = append(args, *filter.SubjectID)
		idx++
	}
	if filter.Name != nil && *filter.Name != "" {
		where = append(where, fmt.Sprintf("name ILIKE $%d", idx))
		args = append(args, "%"+*filter.Name+"%")
		idx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.GetContext(ctx, &total,
		fmt.Sprintf("SELECT COUNT(*) FROM rate_limit_configs %s", whereClause), args...); err != nil {
		return query.Paginated[ratelimit.RateLimitConfig]{}, errx.Wrap(err, "failed to count rate limit configs", errx.TypeInternal)
	}

	dataQ := fmt.Sprintf(
		`SELECT id, name, subject_id, rpm, max_concurrent, created_at, updated_at
		 FROM rate_limit_configs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, idx, idx+1)
	args = append(args, page.Limit, page.Offset)

	var items []ratelimit.RateLimitConfig
	if err := r.db.SelectContext(ctx, &items, dataQ, args...); err != nil {
		return query.Paginated[ratelimit.RateLimitConfig]{}, errx.Wrap(err, "failed to list rate limit configs", errx.TypeInternal)
	}
	if items == nil {
		items = []ratelimit.RateLimitConfig{}
	}

	return query.Paginated[ratelimit.RateLimitConfig]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

// wrapPgError converts common PostgreSQL errors to typed errx errors.
func wrapPgError(err error, entity string) error {
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Code == "23505" { // unique_violation
			return errx.Conflict(entity + " already exists")
		}
	}
	return errx.Wrap(err, "failed to persist "+entity, errx.TypeInternal)
}
