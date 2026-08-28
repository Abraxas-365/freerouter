package guardrails

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

type GuardrailRepository interface {
	// Config
	GetConfig(ctx context.Context, tenantID kernel.TenantID) (*GuardrailConfig, error)
	UpsertConfig(ctx context.Context, config *GuardrailConfig) (*GuardrailConfig, error)

	// Custom rules
	ListRules(ctx context.Context, tenantID kernel.TenantID) ([]*GuardrailRule, error)
	GetRule(ctx context.Context, ruleID kernel.GuardrailRuleID) (*GuardrailRule, error)
	CreateRule(ctx context.Context, rule *GuardrailRule) (*GuardrailRule, error)
	UpdateRule(ctx context.Context, rule *GuardrailRule) (*GuardrailRule, error)
	DeleteRule(ctx context.Context, ruleID kernel.GuardrailRuleID) error

	// Violations
	LogViolation(ctx context.Context, violation *GuardrailViolation) error
	ListViolations(ctx context.Context, tenantID kernel.TenantID, limit, offset int) ([]*GuardrailViolation, int, error)
}
