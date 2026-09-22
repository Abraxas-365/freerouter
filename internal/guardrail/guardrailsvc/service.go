package guardrailsvc

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

var (
	_ guardrail.Commands  = (*Service)(nil)
	_ guardrail.Queries   = (*Service)(nil)
	_ guardrail.Evaluator = (*Service)(nil)
)

// Service implements guardrail config/rule CRUD and request-time evaluation.
type Service struct {
	repo guardrail.Repository
}

// New creates a guardrail service.
func New(repo guardrail.Repository) *Service {
	return &Service{repo: repo}
}

// ── Config ──────────────────────────────────────────────────────────

func (s *Service) GetConfig(ctx context.Context) (guardrail.GuardrailConfig, error) {
	return s.repo.GetConfig(ctx)
}

func (s *Service) UpsertConfig(ctx context.Context, cmd guardrail.UpsertConfig) (guardrail.GuardrailConfig, error) {
	existing, err := s.repo.GetConfig(ctx)
	now := time.Now().UTC()

	if err != nil {
		// No config yet — create with defaults + overrides.
		sysRules := guardrail.DefaultSystemRulesConfig()
		if cmd.SystemRules != nil {
			sysRules = *cmd.SystemRules
		}

		enabled := true
		if cmd.Enabled != nil {
			enabled = *cmd.Enabled
		}

		cfg := guardrail.GuardrailConfig{
			ID:          identity.NewGuardrailConfigID(),
			Enabled:     enabled,
			SystemRules: sysRules,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.repo.UpsertConfig(ctx, cfg); err != nil {
			return guardrail.GuardrailConfig{}, err
		}
		return cfg, nil
	}

	if cmd.Enabled != nil {
		existing.Enabled = *cmd.Enabled
	}
	if cmd.SystemRules != nil {
		existing.SystemRules = *cmd.SystemRules
	}
	existing.UpdatedAt = now

	if err := s.repo.UpsertConfig(ctx, existing); err != nil {
		return guardrail.GuardrailConfig{}, err
	}
	return existing, nil
}

// ── Custom rules ────────────────────────────────────────────────────

func (s *Service) ListRules(ctx context.Context) ([]guardrail.GuardrailRule, error) {
	return s.repo.ListRules(ctx)
}

func (s *Service) FindRule(ctx context.Context, id identity.GuardrailRuleID) (guardrail.GuardrailRule, error) {
	return s.repo.FindRule(ctx, id)
}

func (s *Service) CreateRule(ctx context.Context, cmd guardrail.CreateRule) (guardrail.GuardrailRule, error) {
	if err := cmd.Validate(); err != nil {
		return guardrail.GuardrailRule{}, err
	}

	priority := 100
	if cmd.Priority != nil {
		priority = *cmd.Priority
	}

	now := time.Now().UTC()
	rule := guardrail.GuardrailRule{
		ID:        identity.NewGuardrailRuleID(),
		Name:      cmd.Name,
		Type:      cmd.Type,
		Config:    cmd.Config,
		Priority:  priority,
		Enabled:   true,
		Action:    cmd.Action,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateRule(ctx, rule); err != nil {
		return guardrail.GuardrailRule{}, err
	}
	return rule, nil
}

func (s *Service) UpdateRule(ctx context.Context, id identity.GuardrailRuleID, cmd guardrail.UpdateRule) (guardrail.GuardrailRule, error) {
	if err := cmd.Validate(); err != nil {
		return guardrail.GuardrailRule{}, err
	}

	rule, err := s.repo.FindRule(ctx, id)
	if err != nil {
		return guardrail.GuardrailRule{}, err
	}

	if cmd.Name != nil {
		rule.Name = *cmd.Name
	}
	if cmd.Config != nil {
		rule.Config = *cmd.Config
	}
	if cmd.Priority != nil {
		rule.Priority = *cmd.Priority
	}
	if cmd.Enabled != nil {
		rule.Enabled = *cmd.Enabled
	}
	if cmd.Action != nil {
		rule.Action = *cmd.Action
	}
	rule.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateRule(ctx, rule); err != nil {
		return guardrail.GuardrailRule{}, err
	}
	return rule, nil
}

func (s *Service) DeleteRule(ctx context.Context, id identity.GuardrailRuleID) error {
	return s.repo.DeleteRule(ctx, id)
}

// ── Violations ──────────────────────────────────────────────────────

func (s *Service) ListViolations(ctx context.Context, page query.Pagination) (query.Paginated[guardrail.GuardrailViolation], error) {
	return s.repo.ListViolations(ctx, page)
}

// ── Evaluation ──────────────────────────────────────────────────────

// CheckMessages evaluates message texts against the global config + custom rules.
// It never returns an error for evaluation failures — on any internal problem it
// fails open (Passed: true) so a broken config never takes the gateway down.
func (s *Service) CheckMessages(ctx context.Context, messages []string, modelName string) (*guardrail.CheckResult, error) {
	config, err := s.repo.GetConfig(ctx)
	if err != nil {
		// No config = guardrails disabled, pass through
		return &guardrail.CheckResult{Passed: true}, nil
	}
	if !config.Enabled {
		return &guardrail.CheckResult{Passed: true}, nil
	}

	sysRules := config.SystemRules

	result := &guardrail.CheckResult{Passed: true}

	for i, text := range messages {
		s.checkSystemRules(text, i, sysRules, result)
	}

	customRules, err := s.repo.ListRules(ctx)
	if err == nil {
		for _, text := range messages {
			s.checkCustomRules(text, customRules, result)
		}
	}

	result.Blocked = hasBlockingViolation(result.Violations)
	result.Passed = !result.Blocked

	if len(result.Violations) > 0 {
		go s.logViolations(modelName, result.Violations)
	}

	return result, nil
}

func (s *Service) checkSystemRules(text string, msgIndex int, cfg guardrail.SystemRulesConfig, result *guardrail.CheckResult) {
	if cfg.PromptInjection.Enabled {
		if matched := guardrail.CheckInjection(text); matched != "" {
			result.Violations = append(result.Violations, guardrail.RuleViolation{
				RuleID:         "system:prompt_injection",
				RuleName:       "Prompt Injection Detection",
				Category:       "prompt_injection",
				Action:         cfg.PromptInjection.Action,
				MatchedPattern: "prompt_injection",
				MatchedContent: guardrail.Truncate(matched, 100),
			})
		}
	}

	if cfg.Jailbreak.Enabled {
		if matched := guardrail.CheckJailbreak(text); matched != "" {
			result.Violations = append(result.Violations, guardrail.RuleViolation{
				RuleID:         "system:jailbreak",
				RuleName:       "Jailbreak Prevention",
				Category:       "jailbreak",
				Action:         cfg.Jailbreak.Action,
				MatchedPattern: "jailbreak",
				MatchedContent: guardrail.Truncate(matched, 100),
			})
		}
	}

	if cfg.PIIDetection.Enabled {
		if matches := guardrail.CheckPII(text); len(matches) > 0 {
			result.Violations = append(result.Violations, guardrail.RuleViolation{
				RuleID:         "system:pii_detection",
				RuleName:       "PII Detection",
				Category:       "pii",
				Action:         cfg.PIIDetection.Action,
				MatchedPattern: joinDetectorNames(matches),
			})
			if cfg.PIIDetection.Action == guardrail.ActionRedact {
				result.Redactions = append(result.Redactions, guardrail.RedactionInfo{MessageIndex: msgIndex})
			}
		}
	}

	if cfg.Secrets.Enabled {
		if matches := guardrail.CheckSecrets(text); len(matches) > 0 {
			result.Violations = append(result.Violations, guardrail.RuleViolation{
				RuleID:         "system:secrets",
				RuleName:       "Secrets Detection",
				Category:       "secrets",
				Action:         cfg.Secrets.Action,
				MatchedPattern: joinDetectorNames(matches),
			})
			if cfg.Secrets.Action == guardrail.ActionRedact {
				result.Redactions = append(result.Redactions, guardrail.RedactionInfo{MessageIndex: msgIndex})
			}
		}
	}

	if cfg.DocumentLeakage.Enabled {
		if matched := guardrail.CheckDocumentLeakage(text); matched != "" {
			result.Violations = append(result.Violations, guardrail.RuleViolation{
				RuleID:         "system:document_leakage",
				RuleName:       "Document Leakage Detection",
				Category:       "document_leakage",
				Action:         cfg.DocumentLeakage.Action,
				MatchedPattern: "document_leakage",
				MatchedContent: guardrail.Truncate(matched, 100),
			})
			if cfg.DocumentLeakage.Action == guardrail.ActionRedact {
				result.Redactions = append(result.Redactions, guardrail.RedactionInfo{MessageIndex: msgIndex})
			}
		}
	}
}

func (s *Service) checkCustomRules(text string, rules []guardrail.GuardrailRule, result *guardrail.CheckResult) {
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		var matched string
		switch rule.Type {
		case guardrail.RuleTypeBlockedTerms:
			var cfg guardrail.BlockedTermsConfig
			if err := json.Unmarshal(rule.Config, &cfg); err != nil {
				continue
			}
			matched = guardrail.CheckBlockedTerms(text, cfg)

		case guardrail.RuleTypeCustomRegex:
			var cfg guardrail.CustomRegexConfig
			if err := json.Unmarshal(rule.Config, &cfg); err != nil {
				continue
			}
			matched = guardrail.CheckCustomRegex(text, cfg)
		}

		if matched != "" {
			result.Violations = append(result.Violations, guardrail.RuleViolation{
				RuleID:         rule.ID.String(),
				RuleName:       rule.Name,
				Category:       string(rule.Type),
				Action:         rule.Action,
				MatchedPattern: guardrail.Truncate(matched, 100),
			})
		}
	}
}

func (s *Service) logViolations(modelName string, violations []guardrail.RuleViolation) {
	ctx := context.Background()
	for _, v := range violations {
		actionTaken := "warned"
		switch v.Action {
		case guardrail.ActionBlock:
			actionTaken = "blocked"
		case guardrail.ActionRedact:
			actionTaken = "redacted"
		}

		violation := guardrail.GuardrailViolation{
			ID:             identity.NewGuardrailViolationID(),
			RuleID:         v.RuleID,
			RuleName:       v.RuleName,
			Category:       v.Category,
			ActionTaken:    actionTaken,
			MatchedPattern: v.MatchedPattern,
			MatchedContent: v.MatchedContent,
			Model:          modelName,
			CreatedAt:      time.Now().UTC(),
		}

		if err := s.repo.LogViolation(ctx, violation); err != nil {
			slog.Error("failed to log guardrail violation", "error", err)
		}
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func hasBlockingViolation(violations []guardrail.RuleViolation) bool {
	for _, v := range violations {
		if v.Action == guardrail.ActionBlock {
			return true
		}
	}
	return false
}

func joinDetectorNames(matches []guardrail.DetectorMatch) string {
	seen := map[string]bool{}
	result := ""
	for _, m := range matches {
		if seen[m.Detector] {
			continue
		}
		seen[m.Detector] = true
		if result != "" {
			result += ", "
		}
		result += m.Detector
	}
	return result
}
