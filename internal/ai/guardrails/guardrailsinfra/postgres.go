package guardrailsinfra

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/ai/guardrails"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/jmoiron/sqlx"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetConfig(ctx context.Context, tenantID kernel.TenantID) (*guardrails.GuardrailConfig, error) {
	var config guardrails.GuardrailConfig
	err := r.db.GetContext(ctx, &config, `SELECT * FROM guardrail_configs WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, guardrails.ErrConfigNotFound()
	}
	return &config, nil
}

func (r *PostgresRepository) UpsertConfig(ctx context.Context, config *guardrails.GuardrailConfig) (*guardrails.GuardrailConfig, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO guardrail_configs (id, tenant_id, enabled, system_rules, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			system_rules = EXCLUDED.system_rules,
			updated_at = NOW()
	`, config.ID, config.TenantID, config.Enabled, config.SystemRules)
	if err != nil {
		return nil, err
	}
	return r.GetConfig(ctx, config.TenantID)
}

func (r *PostgresRepository) ListRules(ctx context.Context, tenantID kernel.TenantID) ([]*guardrails.GuardrailRule, error) {
	var rules []*guardrails.GuardrailRule
	err := r.db.SelectContext(ctx, &rules, `
		SELECT * FROM guardrail_rules
		WHERE tenant_id = $1
		ORDER BY priority DESC, created_at ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *PostgresRepository) GetRule(ctx context.Context, ruleID kernel.GuardrailRuleID) (*guardrails.GuardrailRule, error) {
	var rule guardrails.GuardrailRule
	err := r.db.GetContext(ctx, &rule, `SELECT * FROM guardrail_rules WHERE id = $1`, ruleID)
	if err != nil {
		return nil, guardrails.ErrRuleNotFound()
	}
	return &rule, nil
}

func (r *PostgresRepository) CreateRule(ctx context.Context, rule *guardrails.GuardrailRule) (*guardrails.GuardrailRule, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO guardrail_rules (id, tenant_id, name, type, config, priority, enabled, action, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`, rule.ID, rule.TenantID, rule.Name, rule.Type, rule.Config, rule.Priority, rule.Enabled, rule.Action)
	if err != nil {
		return nil, err
	}
	return r.GetRule(ctx, rule.ID)
}

func (r *PostgresRepository) UpdateRule(ctx context.Context, rule *guardrails.GuardrailRule) (*guardrails.GuardrailRule, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE guardrail_rules
		SET name = $2, config = $3, priority = $4, enabled = $5, action = $6, updated_at = NOW()
		WHERE id = $1
	`, rule.ID, rule.Name, rule.Config, rule.Priority, rule.Enabled, rule.Action)
	if err != nil {
		return nil, err
	}
	return r.GetRule(ctx, rule.ID)
}

func (r *PostgresRepository) DeleteRule(ctx context.Context, ruleID kernel.GuardrailRuleID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM guardrail_rules WHERE id = $1`, ruleID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return guardrails.ErrRuleNotFound()
	}
	return nil
}

func (r *PostgresRepository) LogViolation(ctx context.Context, violation *guardrails.GuardrailViolation) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO guardrail_violations (id, tenant_id, rule_id, rule_name, category, action_taken, matched_pattern, matched_content, model, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`, violation.ID, violation.TenantID, violation.RuleID, violation.RuleName, violation.Category,
		violation.ActionTaken, violation.MatchedPattern, violation.MatchedContent, violation.Model)
	return err
}

func (r *PostgresRepository) ListViolations(ctx context.Context, tenantID kernel.TenantID, limit, offset int) ([]*guardrails.GuardrailViolation, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM guardrail_violations WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, 0, err
	}

	var violations []*guardrails.GuardrailViolation
	err = r.db.SelectContext(ctx, &violations, `
		SELECT * FROM guardrail_violations
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return violations, total, nil
}
