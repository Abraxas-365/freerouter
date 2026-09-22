package guardrail

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// ── Commands ────────────────────────────────────────────────────────

// Commands defines write operations for guardrail configuration and rules.
type Commands interface {
	UpsertConfig(ctx context.Context, cmd UpsertConfig) (GuardrailConfig, error)

	CreateRule(ctx context.Context, cmd CreateRule) (GuardrailRule, error)
	UpdateRule(ctx context.Context, id identity.GuardrailRuleID, cmd UpdateRule) (GuardrailRule, error)
	DeleteRule(ctx context.Context, id identity.GuardrailRuleID) error
}

// ── Queries ─────────────────────────────────────────────────────────

// Queries defines read operations for guardrail configuration, rules, and violations.
type Queries interface {
	GetConfig(ctx context.Context) (GuardrailConfig, error)

	ListRules(ctx context.Context) ([]GuardrailRule, error)
	FindRule(ctx context.Context, id identity.GuardrailRuleID) (GuardrailRule, error)

	ListViolations(ctx context.Context, page query.Pagination) (query.Paginated[GuardrailViolation], error)
}

// Evaluator is the gateway-facing check used before routing a request.
type Evaluator interface {
	// CheckMessages evaluates message texts against the active config + rules.
	// modelName is used only for violation logging context.
	CheckMessages(ctx context.Context, messages []string, modelName string) (*CheckResult, error)
}

// ── Repository ──────────────────────────────────────────────────────

// Repository is the persistence contract for guardrail config, rules, and violations.
type Repository interface {
	// Config — single row, upserted.
	GetConfig(ctx context.Context) (GuardrailConfig, error)
	UpsertConfig(ctx context.Context, cfg GuardrailConfig) error

	// Custom rules
	ListRules(ctx context.Context) ([]GuardrailRule, error)
	FindRule(ctx context.Context, id identity.GuardrailRuleID) (GuardrailRule, error)
	CreateRule(ctx context.Context, rule GuardrailRule) error
	UpdateRule(ctx context.Context, rule GuardrailRule) error
	DeleteRule(ctx context.Context, id identity.GuardrailRuleID) error

	// Violations
	LogViolation(ctx context.Context, violation GuardrailViolation) error
	ListViolations(ctx context.Context, page query.Pagination) (query.Paginated[GuardrailViolation], error)
}
