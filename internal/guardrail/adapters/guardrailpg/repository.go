package guardrailpg

import (
	"context"
	"database/sql"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository implements guardrail.Repository with PostgreSQL.
// Config is a single global row (id fixed at "singleton" via unique constraint
// enforced by always upserting the same row).
type Repository struct{ db *sqlx.DB }

var _ guardrail.Repository = (*Repository)(nil)

// New creates a guardrail repository.
func New(db *sqlx.DB) *Repository { return &Repository{db: db} }

// ── Config ──────────────────────────────────────────────────────────

func (r *Repository) GetConfig(ctx context.Context) (guardrail.GuardrailConfig, error) {
	var cfg guardrail.GuardrailConfig
	err := r.db.GetContext(ctx, &cfg,
		`SELECT id, enabled, system_rules, created_at, updated_at
		 FROM guardrail_configs ORDER BY created_at ASC LIMIT 1`)
	if err != nil {
		if err == sql.ErrNoRows {
			return cfg, errx.NotFound("guardrail config not found")
		}
		return cfg, errx.Wrap(err, "failed to get guardrail config", errx.TypeInternal)
	}
	return cfg, nil
}

func (r *Repository) UpsertConfig(ctx context.Context, cfg guardrail.GuardrailConfig) error {
	// Global singleton: try update first, insert if no row exists.
	res, err := r.db.ExecContext(ctx,
		`UPDATE guardrail_configs SET enabled = $2, system_rules = $3, updated_at = $4 WHERE id = $1`,
		cfg.ID, cfg.Enabled, cfg.SystemRules, cfg.UpdatedAt)
	if err != nil {
		return errx.Wrap(err, "failed to update guardrail config", errx.TypeInternal)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return nil
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO guardrail_configs (id, enabled, system_rules, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		cfg.ID, cfg.Enabled, cfg.SystemRules, cfg.CreatedAt, cfg.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "guardrail config")
	}
	return nil
}

// ── Custom rules ────────────────────────────────────────────────────

func (r *Repository) ListRules(ctx context.Context) ([]guardrail.GuardrailRule, error) {
	var rules []guardrail.GuardrailRule
	err := r.db.SelectContext(ctx, &rules,
		`SELECT id, name, type, config, priority, enabled, action, created_at, updated_at
		 FROM guardrail_rules ORDER BY priority ASC, created_at ASC`)
	if err != nil {
		return nil, errx.Wrap(err, "failed to list guardrail rules", errx.TypeInternal)
	}
	if rules == nil {
		rules = []guardrail.GuardrailRule{}
	}
	return rules, nil
}

func (r *Repository) FindRule(ctx context.Context, id identity.GuardrailRuleID) (guardrail.GuardrailRule, error) {
	var rule guardrail.GuardrailRule
	err := r.db.GetContext(ctx, &rule,
		`SELECT id, name, type, config, priority, enabled, action, created_at, updated_at
		 FROM guardrail_rules WHERE id = $1`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return rule, errx.NotFound("guardrail rule not found")
		}
		return rule, errx.Wrap(err, "failed to find guardrail rule", errx.TypeInternal)
	}
	return rule, nil
}

func (r *Repository) CreateRule(ctx context.Context, rule guardrail.GuardrailRule) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO guardrail_rules (id, name, type, config, priority, enabled, action, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		rule.ID, rule.Name, rule.Type, rule.Config, rule.Priority, rule.Enabled, rule.Action, rule.CreatedAt, rule.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "guardrail rule")
	}
	return nil
}

func (r *Repository) UpdateRule(ctx context.Context, rule guardrail.GuardrailRule) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE guardrail_rules
		 SET name = $2, config = $3, priority = $4, enabled = $5, action = $6, updated_at = $7
		 WHERE id = $1`,
		rule.ID, rule.Name, rule.Config, rule.Priority, rule.Enabled, rule.Action, rule.UpdatedAt)
	if err != nil {
		return wrapPgError(err, "guardrail rule")
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errx.NotFound("guardrail rule not found")
	}
	return nil
}

func (r *Repository) DeleteRule(ctx context.Context, id identity.GuardrailRuleID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM guardrail_rules WHERE id = $1`, id)
	if err != nil {
		return errx.Wrap(err, "failed to delete guardrail rule", errx.TypeInternal)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errx.NotFound("guardrail rule not found")
	}
	return nil
}

// ── Violations ──────────────────────────────────────────────────────

func (r *Repository) LogViolation(ctx context.Context, v guardrail.GuardrailViolation) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO guardrail_violations (id, rule_id, rule_name, category, action_taken, matched_pattern, matched_content, model, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		v.ID, v.RuleID, v.RuleName, v.Category, v.ActionTaken, nullableString(v.MatchedPattern), nullableString(v.MatchedContent), nullableString(v.Model), v.CreatedAt)
	if err != nil {
		return errx.Wrap(err, "failed to log guardrail violation", errx.TypeInternal)
	}
	return nil
}

func (r *Repository) ListViolations(ctx context.Context, page query.Pagination) (query.Paginated[guardrail.GuardrailViolation], error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM guardrail_violations`); err != nil {
		return query.Paginated[guardrail.GuardrailViolation]{}, errx.Wrap(err, "failed to count guardrail violations", errx.TypeInternal)
	}

	var items []guardrail.GuardrailViolation
	err := r.db.SelectContext(ctx, &items,
		`SELECT id, rule_id, rule_name, category, action_taken,
		        COALESCE(matched_pattern, '') AS matched_pattern,
		        COALESCE(matched_content, '') AS matched_content,
		        COALESCE(model, '') AS model,
		        created_at
		 FROM guardrail_violations ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		page.Limit, page.Offset)
	if err != nil {
		return query.Paginated[guardrail.GuardrailViolation]{}, errx.Wrap(err, "failed to list guardrail violations", errx.TypeInternal)
	}
	if items == nil {
		items = []guardrail.GuardrailViolation{}
	}

	return query.Paginated[guardrail.GuardrailViolation]{
		Items: items,
		Page:  query.Page{Total: total, Limit: page.Limit, Offset: page.Offset},
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func wrapPgError(err error, entity string) error {
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Code == "23505" { // unique_violation
			return errx.Conflict(entity + " already exists")
		}
	}
	return errx.Wrap(err, "failed to persist "+entity, errx.TypeInternal)
}
