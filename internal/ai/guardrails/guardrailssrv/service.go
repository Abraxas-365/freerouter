package guardrailssrv

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Abraxas-365/freerouter/internal/ai/guardrails"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/google/uuid"
)

type GuardrailsService struct {
	repo guardrails.GuardrailRepository
}

func New(repo guardrails.GuardrailRepository) *GuardrailsService {
	return &GuardrailsService{repo: repo}
}

// CheckMessages checks all messages against the tenant's guardrail config.
// Returns the check result with violations and redactions to apply.
func (s *GuardrailsService) CheckMessages(ctx context.Context, tenantID kernel.TenantID, messages []string, model string) (*guardrails.CheckResult, error) {
	config, err := s.repo.GetConfig(ctx, tenantID)
	if err != nil {
		// No config = guardrails disabled, pass through
		return &guardrails.CheckResult{Passed: true}, nil
	}
	if !config.Enabled {
		return &guardrails.CheckResult{Passed: true}, nil
	}

	sysRules, err := config.ParseSystemRules()
	if err != nil {
		slog.Error("failed to parse system rules config", "tenant_id", tenantID, "error", err)
		return &guardrails.CheckResult{Passed: true}, nil
	}

	result := &guardrails.CheckResult{Passed: true}

	// Check system rules on each message
	for i, text := range messages {
		s.checkSystemRules(text, i, sysRules, result)
	}

	// Check custom rules on each message
	customRules, err := s.repo.ListRules(ctx, tenantID)
	if err == nil {
		for _, text := range messages {
			s.checkCustomRules(text, customRules, result)
		}
	}

	result.Blocked = hasBlockingViolation(result.Violations)
	result.Passed = !result.Blocked

	// Log violations asynchronously (best effort)
	if len(result.Violations) > 0 {
		go s.logViolations(tenantID, model, result.Violations)
	}

	return result, nil
}

func (s *GuardrailsService) checkSystemRules(text string, msgIndex int, cfg guardrails.SystemRulesConfig, result *guardrails.CheckResult) {
	// Prompt injection
	if cfg.PromptInjection.Enabled {
		if matched := guardrails.CheckInjection(text); matched != "" {
			result.Violations = append(result.Violations, guardrails.RuleViolation{
				RuleID:         "system:prompt_injection",
				RuleName:       "Prompt Injection Detection",
				Category:       "prompt_injection",
				Action:         cfg.PromptInjection.Action,
				MatchedPattern: "prompt_injection",
				MatchedContent: guardrails.Truncate(matched, 100),
			})
		}
	}

	// Jailbreak
	if cfg.Jailbreak.Enabled {
		if matched := guardrails.CheckJailbreak(text); matched != "" {
			result.Violations = append(result.Violations, guardrails.RuleViolation{
				RuleID:         "system:jailbreak",
				RuleName:       "Jailbreak Prevention",
				Category:       "jailbreak",
				Action:         cfg.Jailbreak.Action,
				MatchedPattern: "jailbreak",
				MatchedContent: guardrails.Truncate(matched, 100),
			})
		}
	}

	// PII detection
	if cfg.PIIDetection.Enabled {
		matches := guardrails.CheckPII(text)
		if len(matches) > 0 {
			// Collect detector names, not raw values
			var detectors []string
			seen := map[string]bool{}
			for _, m := range matches {
				if !seen[m.Detector] {
					detectors = append(detectors, m.Detector)
					seen[m.Detector] = true
				}
			}

			result.Violations = append(result.Violations, guardrails.RuleViolation{
				RuleID:         "system:pii_detection",
				RuleName:       "PII Detection",
				Category:       "pii",
				Action:         cfg.PIIDetection.Action,
				MatchedPattern: joinStrings(detectors),
			})

			if cfg.PIIDetection.Action == guardrails.ActionRedact {
				result.Redactions = append(result.Redactions, guardrails.RedactionInfo{
					MessageIndex: msgIndex,
					Matches:      detectorMatchValues(matches),
					Replacement:  "", // uses per-match replacement
				})
			}
		}
	}

	// Secrets detection
	if cfg.Secrets.Enabled {
		matches := guardrails.CheckSecrets(text)
		if len(matches) > 0 {
			var detectors []string
			seen := map[string]bool{}
			for _, m := range matches {
				if !seen[m.Detector] {
					detectors = append(detectors, m.Detector)
					seen[m.Detector] = true
				}
			}

			result.Violations = append(result.Violations, guardrails.RuleViolation{
				RuleID:         "system:secrets",
				RuleName:       "Secrets Detection",
				Category:       "secrets",
				Action:         cfg.Secrets.Action,
				MatchedPattern: joinStrings(detectors),
			})

			if cfg.Secrets.Action == guardrails.ActionRedact {
				result.Redactions = append(result.Redactions, guardrails.RedactionInfo{
					MessageIndex: msgIndex,
					Matches:      detectorMatchValues(matches),
					Replacement:  "", // uses per-match replacement
				})
			}
		}
	}

	// Document leakage
	if cfg.DocumentLeakage.Enabled {
		if matched := guardrails.CheckDocumentLeakage(text); matched != "" {
			result.Violations = append(result.Violations, guardrails.RuleViolation{
				RuleID:         "system:document_leakage",
				RuleName:       "Document Leakage Detection",
				Category:       "document_leakage",
				Action:         cfg.DocumentLeakage.Action,
				MatchedPattern: "document_leakage",
				MatchedContent: guardrails.Truncate(matched, 100),
			})
		}
	}
}

func (s *GuardrailsService) checkCustomRules(text string, rules []*guardrails.GuardrailRule, result *guardrails.CheckResult) {
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		var matched string
		switch rule.Type {
		case guardrails.RuleTypeBlockedTerms:
			var cfg guardrails.BlockedTermsConfig
			if err := json.Unmarshal(rule.Config, &cfg); err != nil {
				continue
			}
			matched = guardrails.CheckBlockedTerms(text, cfg)

		case guardrails.RuleTypeCustomRegex:
			var cfg guardrails.CustomRegexConfig
			if err := json.Unmarshal(rule.Config, &cfg); err != nil {
				continue
			}
			matched = guardrails.CheckCustomRegex(text, cfg)
		}

		if matched != "" {
			result.Violations = append(result.Violations, guardrails.RuleViolation{
				RuleID:         rule.ID.String(),
				RuleName:       rule.Name,
				Category:       string(rule.Type),
				Action:         rule.Action,
				MatchedPattern: guardrails.Truncate(matched, 100),
			})
		}
	}
}

func (s *GuardrailsService) logViolations(tenantID kernel.TenantID, model string, violations []guardrails.RuleViolation) {
	ctx := context.Background()
	var modelPtr *string
	if model != "" {
		modelPtr = &model
	}

	for _, v := range violations {
		actionTaken := "warned"
		switch v.Action {
		case guardrails.ActionBlock:
			actionTaken = "blocked"
		case guardrails.ActionRedact:
			actionTaken = "redacted"
		}

		var matchedPattern, matchedContent *string
		if v.MatchedPattern != "" {
			s := v.MatchedPattern
			matchedPattern = &s
		}
		if v.MatchedContent != "" {
			s := v.MatchedContent
			matchedContent = &s
		}

		violation := &guardrails.GuardrailViolation{
			ID:             kernel.NewGuardrailViolationID(uuid.New().String()),
			TenantID:       tenantID,
			RuleID:         v.RuleID,
			RuleName:       v.RuleName,
			Category:       v.Category,
			ActionTaken:    actionTaken,
			MatchedPattern: matchedPattern,
			MatchedContent: matchedContent,
			Model:          modelPtr,
		}

		if err := s.repo.LogViolation(ctx, violation); err != nil {
			slog.Error("failed to log guardrail violation", "error", err)
		}
	}
}

// ============================================================================
// Config + Rule CRUD (delegated to repo)
// ============================================================================

func (s *GuardrailsService) GetConfig(ctx context.Context, tenantID kernel.TenantID) (*guardrails.GuardrailConfig, error) {
	return s.repo.GetConfig(ctx, tenantID)
}

func (s *GuardrailsService) UpsertConfig(ctx context.Context, tenantID kernel.TenantID, req guardrails.UpsertConfigRequest) (*guardrails.GuardrailConfig, error) {
	existing, err := s.repo.GetConfig(ctx, tenantID)
	if err != nil {
		// Create new with defaults
		sysRules := guardrails.DefaultSystemRulesConfig()
		if req.SystemRules != nil {
			sysRules = *req.SystemRules
		}
		sysJSON, _ := json.Marshal(sysRules)

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		config := &guardrails.GuardrailConfig{
			ID:          kernel.NewGuardrailConfigID(uuid.New().String()),
			TenantID:    tenantID,
			Enabled:     enabled,
			SystemRules: sysJSON,
		}
		return s.repo.UpsertConfig(ctx, config)
	}

	// Update existing
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.SystemRules != nil {
		sysJSON, _ := json.Marshal(req.SystemRules)
		existing.SystemRules = sysJSON
	}
	return s.repo.UpsertConfig(ctx, existing)
}

func (s *GuardrailsService) ListRules(ctx context.Context, tenantID kernel.TenantID) ([]*guardrails.GuardrailRule, error) {
	return s.repo.ListRules(ctx, tenantID)
}

func (s *GuardrailsService) CreateRule(ctx context.Context, tenantID kernel.TenantID, req guardrails.CreateRuleRequest) (*guardrails.GuardrailRule, error) {
	priority := 100
	if req.Priority != nil {
		priority = *req.Priority
	}

	rule := &guardrails.GuardrailRule{
		ID:       kernel.NewGuardrailRuleID(uuid.New().String()),
		TenantID: tenantID,
		Name:     req.Name,
		Type:     req.Type,
		Config:   req.Config,
		Priority: priority,
		Enabled:  true,
		Action:   req.Action,
	}
	return s.repo.CreateRule(ctx, rule)
}

func (s *GuardrailsService) UpdateRule(ctx context.Context, ruleID kernel.GuardrailRuleID, req guardrails.UpdateRuleRequest) (*guardrails.GuardrailRule, error) {
	rule, err := s.repo.GetRule(ctx, ruleID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Config != nil {
		rule.Config = *req.Config
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Action != nil {
		rule.Action = *req.Action
	}

	return s.repo.UpdateRule(ctx, rule)
}

func (s *GuardrailsService) DeleteRule(ctx context.Context, ruleID kernel.GuardrailRuleID) error {
	return s.repo.DeleteRule(ctx, ruleID)
}

func (s *GuardrailsService) ListViolations(ctx context.Context, tenantID kernel.TenantID, limit, offset int) ([]*guardrails.GuardrailViolation, int, error) {
	return s.repo.ListViolations(ctx, tenantID, limit, offset)
}

// ============================================================================
// Helpers
// ============================================================================

func hasBlockingViolation(violations []guardrails.RuleViolation) bool {
	for _, v := range violations {
		if v.Action == guardrails.ActionBlock {
			return true
		}
	}
	return false
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

func detectorMatchValues(matches []guardrails.DetectorMatch) []string {
	vals := make([]string, len(matches))
	for i, m := range matches {
		vals[i] = m.Value
	}
	return vals
}
