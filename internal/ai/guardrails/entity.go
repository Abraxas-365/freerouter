package guardrails

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Actions
// ============================================================================

type Action string

const (
	ActionBlock  Action = "block"
	ActionRedact Action = "redact"
	ActionWarn   Action = "warn"
	ActionAllow  Action = "allow"
)

// ============================================================================
// System rules config
// ============================================================================

type SystemRuleConfig struct {
	Enabled bool   `json:"enabled"`
	Action  Action `json:"action"`
}

type SystemRulesConfig struct {
	PromptInjection SystemRuleConfig `json:"prompt_injection"`
	Jailbreak       SystemRuleConfig `json:"jailbreak"`
	PIIDetection    SystemRuleConfig `json:"pii_detection"`
	Secrets         SystemRuleConfig `json:"secrets"`
	DocumentLeakage SystemRuleConfig `json:"document_leakage"`
}

func DefaultSystemRulesConfig() SystemRulesConfig {
	return SystemRulesConfig{
		PromptInjection: SystemRuleConfig{Enabled: true, Action: ActionBlock},
		Jailbreak:       SystemRuleConfig{Enabled: true, Action: ActionBlock},
		PIIDetection:    SystemRuleConfig{Enabled: true, Action: ActionRedact},
		Secrets:         SystemRuleConfig{Enabled: true, Action: ActionBlock},
		DocumentLeakage: SystemRuleConfig{Enabled: false, Action: ActionWarn},
	}
}

// ============================================================================
// Guardrail Config (per-tenant)
// ============================================================================

type GuardrailConfig struct {
	ID          kernel.GuardrailConfigID `db:"id" json:"id"`
	TenantID    kernel.TenantID          `db:"tenant_id" json:"tenant_id"`
	Enabled     bool                     `db:"enabled" json:"enabled"`
	SystemRules json.RawMessage          `db:"system_rules" json:"system_rules"` // SystemRulesConfig as JSON
	CreatedAt   time.Time                `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time                `db:"updated_at" json:"updated_at"`
}

func (c *GuardrailConfig) ParseSystemRules() (SystemRulesConfig, error) {
	var cfg SystemRulesConfig
	if len(c.SystemRules) == 0 {
		return DefaultSystemRulesConfig(), nil
	}
	if err := json.Unmarshal(c.SystemRules, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// ============================================================================
// Custom guardrail rules
// ============================================================================

type RuleType string

const (
	RuleTypeBlockedTerms RuleType = "blocked_terms"
	RuleTypeCustomRegex  RuleType = "custom_regex"
)

type GuardrailRule struct {
	ID       kernel.GuardrailRuleID `db:"id" json:"id"`
	TenantID kernel.TenantID        `db:"tenant_id" json:"tenant_id"`
	Name     string                 `db:"name" json:"name"`
	Type     RuleType               `db:"type" json:"type"`
	Config   json.RawMessage        `db:"config" json:"config"` // BlockedTermsConfig or CustomRegexConfig
	Priority int                    `db:"priority" json:"priority"`
	Enabled  bool                   `db:"enabled" json:"enabled"`
	Action   Action                 `db:"action" json:"action"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type BlockedTermsConfig struct {
	Terms         []string `json:"terms"`
	MatchType     string   `json:"match_type"` // "exact", "contains", "regex"
	CaseSensitive bool     `json:"case_sensitive"`
}

type CustomRegexConfig struct {
	Pattern string `json:"pattern"`
}

// ============================================================================
// Violations
// ============================================================================

type GuardrailViolation struct {
	ID             kernel.GuardrailViolationID `db:"id" json:"id"`
	TenantID       kernel.TenantID             `db:"tenant_id" json:"tenant_id"`
	RuleID         string                      `db:"rule_id" json:"rule_id"`
	RuleName       string                      `db:"rule_name" json:"rule_name"`
	Category       string                      `db:"category" json:"category"`
	ActionTaken    string                      `db:"action_taken" json:"action_taken"` // "blocked", "redacted", "warned"
	MatchedPattern *string                     `db:"matched_pattern" json:"matched_pattern,omitempty"`
	MatchedContent *string                     `db:"matched_content" json:"matched_content,omitempty"`
	Model          *string                     `db:"model" json:"model,omitempty"`
	CreatedAt      time.Time                   `db:"created_at" json:"created_at"`
}

// ============================================================================
// Check result
// ============================================================================

type RuleViolation struct {
	RuleID         string `json:"rule_id"`
	RuleName       string `json:"rule_name"`
	Category       string `json:"category"`
	Action         Action `json:"action"`
	MatchedPattern string `json:"matched_pattern,omitempty"`
	MatchedContent string `json:"matched_content,omitempty"`
}

type RedactionInfo struct {
	MessageIndex int      `json:"message_index"`
	Matches      []string `json:"matches"`
	Replacement  string   `json:"replacement"`
}

type CheckResult struct {
	Passed     bool            `json:"passed"`
	Blocked    bool            `json:"blocked"`
	Violations []RuleViolation `json:"violations"`
	Redactions []RedactionInfo `json:"redactions"`
}

// ============================================================================
// Request types
// ============================================================================

type UpsertConfigRequest struct {
	Enabled     *bool              `json:"enabled,omitempty"`
	SystemRules *SystemRulesConfig `json:"system_rules,omitempty"`
}

type CreateRuleRequest struct {
	Name     string          `json:"name"`
	Type     RuleType        `json:"type"`
	Config   json.RawMessage `json:"config"`
	Priority *int            `json:"priority,omitempty"`
	Action   Action          `json:"action"`
}

func (r *CreateRuleRequest) Validate() error {
	if r.Name == "" {
		return errx.Validation("name is required").WithDetail("field", "name")
	}
	switch r.Type {
	case RuleTypeBlockedTerms, RuleTypeCustomRegex:
	default:
		return errx.Validation("type must be blocked_terms or custom_regex").WithDetail("field", "type")
	}
	switch r.Action {
	case ActionBlock, ActionRedact, ActionWarn:
	default:
		return errx.Validation("action must be block, redact, or warn").WithDetail("field", "action")
	}
	if len(r.Config) == 0 {
		return errx.Validation("config is required").WithDetail("field", "config")
	}
	return nil
}

type UpdateRuleRequest struct {
	Name     *string          `json:"name,omitempty"`
	Config   *json.RawMessage `json:"config,omitempty"`
	Priority *int             `json:"priority,omitempty"`
	Enabled  *bool            `json:"enabled,omitempty"`
	Action   *Action          `json:"action,omitempty"`
}

// ============================================================================
// Errors
// ============================================================================

var ErrRegistry = errx.NewRegistry("GUARDRAIL")

var (
	CodeConfigNotFound = ErrRegistry.Register("CONFIG_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Guardrail config not found")
	CodeRuleNotFound   = ErrRegistry.Register("RULE_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Guardrail rule not found")
	CodeContentBlocked = ErrRegistry.Register("CONTENT_BLOCKED", errx.TypeBusiness, http.StatusBadRequest, "Request blocked by content policy")
)

func ErrConfigNotFound() *errx.Error { return ErrRegistry.New(CodeConfigNotFound) }
func ErrRuleNotFound() *errx.Error   { return ErrRegistry.New(CodeRuleNotFound) }
func ErrContentBlocked() *errx.Error { return ErrRegistry.New(CodeContentBlocked) }
