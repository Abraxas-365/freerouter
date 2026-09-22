package guardrail

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Action ──────────────────────────────────────────────────────────

// Action describes what happens when a rule matches.
type Action string

const (
	ActionBlock  Action = "block"
	ActionRedact Action = "redact"
	ActionWarn   Action = "warn"
)

func (a Action) Valid() bool {
	switch a {
	case ActionBlock, ActionRedact, ActionWarn:
		return true
	}
	return false
}

// ── System rules ────────────────────────────────────────────────────

// SystemRuleConfig toggles and configures one built-in detector.
type SystemRuleConfig struct {
	Enabled bool   `json:"enabled"`
	Action  Action `json:"action"`
}

// SystemRulesConfig holds settings for all built-in detectors.
//
// It implements sql.Scanner/driver.Valuer so it can be stored directly as a
// JSONB column (see migrations/006_guardrails.up.sql) without going through
// an intermediate json.RawMessage field — GuardrailConfig.SystemRules is
// typed end to end, no ad-hoc unmarshal at the call site.
type SystemRulesConfig struct {
	PromptInjection SystemRuleConfig `json:"prompt_injection"`
	Jailbreak       SystemRuleConfig `json:"jailbreak"`
	PIIDetection    SystemRuleConfig `json:"pii_detection"`
	Secrets         SystemRuleConfig `json:"secrets"`
	DocumentLeakage SystemRuleConfig `json:"document_leakage"`
}

// DefaultSystemRulesConfig returns sane defaults: block injection/jailbreak/secrets, redact PII.
func DefaultSystemRulesConfig() SystemRulesConfig {
	return SystemRulesConfig{
		PromptInjection: SystemRuleConfig{Enabled: true, Action: ActionBlock},
		Jailbreak:       SystemRuleConfig{Enabled: true, Action: ActionBlock},
		PIIDetection:    SystemRuleConfig{Enabled: true, Action: ActionRedact},
		Secrets:         SystemRuleConfig{Enabled: true, Action: ActionBlock},
		DocumentLeakage: SystemRuleConfig{Enabled: false, Action: ActionWarn},
	}
}

// Scan implements sql.Scanner, decoding a JSON/JSONB column. A NULL or empty
// value falls back to DefaultSystemRulesConfig() so the field is always usable.
func (c *SystemRulesConfig) Scan(src any) error {
	if src == nil {
		*c = DefaultSystemRulesConfig()
		return nil
	}

	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("guardrail: cannot scan %T into SystemRulesConfig", src)
	}

	if len(raw) == 0 {
		*c = DefaultSystemRulesConfig()
		return nil
	}

	var cfg SystemRulesConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("guardrail: failed to unmarshal SystemRulesConfig: %w", err)
	}
	*c = cfg
	return nil
}

// Value implements driver.Valuer, encoding the config as JSON for storage.
func (c SystemRulesConfig) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ── GuardrailConfig ─────────────────────────────────────────────────

// GuardrailConfig is the single global guardrail configuration: enable/disable
// and per-detector system rules. There is exactly one config for the whole
// gateway (no per-tenant scoping in v2 — IAMKit handles multi-tenancy upstream).
type GuardrailConfig struct {
	ID          identity.GuardrailConfigID `json:"id"           db:"id"`
	Enabled     bool                       `json:"enabled"      db:"enabled"`
	SystemRules SystemRulesConfig          `json:"system_rules" db:"system_rules"`
	CreatedAt   time.Time                  `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"   db:"updated_at"`
}

// UpsertConfig is the input for creating/updating the global config.
type UpsertConfig struct {
	Enabled     *bool              `json:"enabled,omitempty"`
	SystemRules *SystemRulesConfig `json:"system_rules,omitempty"`
}

// ── Custom rules ────────────────────────────────────────────────────

// RuleType identifies the kind of custom rule.
type RuleType string

const (
	RuleTypeBlockedTerms RuleType = "blocked_terms"
	RuleTypeCustomRegex  RuleType = "custom_regex"
)

func (t RuleType) Valid() bool {
	switch t {
	case RuleTypeBlockedTerms, RuleTypeCustomRegex:
		return true
	}
	return false
}

// GuardrailRule is a user-defined content rule (blocked terms or custom regex).
type GuardrailRule struct {
	ID        identity.GuardrailRuleID `json:"id"         db:"id"`
	Name      string                   `json:"name"       db:"name"`
	Type      RuleType                 `json:"type"       db:"type"`
	Config    json.RawMessage          `json:"config"     db:"config"`
	Priority  int                      `json:"priority"   db:"priority"`
	Enabled   bool                     `json:"enabled"    db:"enabled"`
	Action    Action                   `json:"action"     db:"action"`
	CreatedAt time.Time                `json:"created_at" db:"created_at"`
	UpdatedAt time.Time                `json:"updated_at" db:"updated_at"`
}

// BlockedTermsConfig is the Config payload for RuleTypeBlockedTerms.
type BlockedTermsConfig struct {
	Terms         []string `json:"terms"`
	MatchType     string   `json:"match_type"` // "exact", "contains", "regex"
	CaseSensitive bool     `json:"case_sensitive"`
}

// CustomRegexConfig is the Config payload for RuleTypeCustomRegex.
type CustomRegexConfig struct {
	Pattern string `json:"pattern"`
}

// CreateRule is the input for creating a custom rule.
type CreateRule struct {
	Name     string          `json:"name"`
	Type     RuleType        `json:"type"`
	Config   json.RawMessage `json:"config"`
	Priority *int            `json:"priority,omitempty"`
	Action   Action          `json:"action"`
}

// Validate checks required fields.
func (r CreateRule) Validate() error {
	if r.Name == "" {
		return errx.Validation("name is required")
	}
	if !r.Type.Valid() {
		return errx.Validation("type must be blocked_terms or custom_regex")
	}
	if !r.Action.Valid() {
		return errx.Validation("action must be block, redact, or warn")
	}
	if len(r.Config) == 0 {
		return errx.Validation("config is required")
	}
	return nil
}

// UpdateRule holds optional fields for updating a custom rule.
type UpdateRule struct {
	Name     *string          `json:"name,omitempty"`
	Config   *json.RawMessage `json:"config,omitempty"`
	Priority *int             `json:"priority,omitempty"`
	Enabled  *bool            `json:"enabled,omitempty"`
	Action   *Action          `json:"action,omitempty"`
}

// Validate checks update fields if provided.
func (u UpdateRule) Validate() error {
	if u.Name != nil && *u.Name == "" {
		return errx.Validation("name cannot be empty")
	}
	if u.Action != nil && !u.Action.Valid() {
		return errx.Validation("action must be block, redact, or warn")
	}
	return nil
}

// ── Violations ──────────────────────────────────────────────────────

// GuardrailViolation records a single triggered rule for observability.
type GuardrailViolation struct {
	ID             identity.GuardrailViolationID `json:"id"                        db:"id"`
	RuleID         string                        `json:"rule_id"                   db:"rule_id"`
	RuleName       string                        `json:"rule_name"                 db:"rule_name"`
	Category       string                        `json:"category"                  db:"category"`
	ActionTaken    string                        `json:"action_taken"              db:"action_taken"` // "blocked", "redacted", "warned"
	MatchedPattern string                        `json:"matched_pattern,omitempty" db:"matched_pattern"`
	MatchedContent string                        `json:"matched_content,omitempty" db:"matched_content"`
	Model          string                        `json:"model,omitempty"           db:"model"`
	CreatedAt      time.Time                     `json:"created_at"                db:"created_at"`
}

// ── Check result ────────────────────────────────────────────────────

// RuleViolation is an in-flight match produced during evaluation.
type RuleViolation struct {
	RuleID         string `json:"rule_id"`
	RuleName       string `json:"rule_name"`
	Category       string `json:"category"`
	Action         Action `json:"action"`
	MatchedPattern string `json:"matched_pattern,omitempty"`
	MatchedContent string `json:"matched_content,omitempty"`
}

// RedactionInfo describes which message needs redaction and how.
type RedactionInfo struct {
	MessageIndex int `json:"message_index"`
}

// CheckResult is returned by CheckMessages.
type CheckResult struct {
	Passed     bool            `json:"passed"`
	Blocked    bool            `json:"blocked"`
	Violations []RuleViolation `json:"violations"`
	Redactions []RedactionInfo `json:"redactions"`
}
