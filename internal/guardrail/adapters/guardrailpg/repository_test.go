package guardrailpg_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/guardrail"
	"github.com/Abraxas-365/freerouter/internal/guardrail/adapters/guardrailpg"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/testutil"
)

// ── Config (global singleton) ───────────────────────────────────────

func TestRepository_UpsertConfig_InsertsWhenMissing(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := guardrail.GuardrailConfig{
		ID:          identity.NewGuardrailConfigID(),
		Enabled:     true,
		SystemRules: guardrail.DefaultSystemRulesConfig(),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := repo.UpsertConfig(ctx, cfg); err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	got, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got.ID != cfg.ID || !got.Enabled {
		t.Fatalf("unexpected config: %+v", got)
	}

	if !got.SystemRules.PromptInjection.Enabled {
		t.Fatalf("expected prompt injection rule enabled by default")
	}
}

func TestRepository_UpsertConfig_UpdatesExistingRow(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)
	ctx := context.Background()

	id := identity.NewGuardrailConfigID()
	now := time.Now().UTC().Truncate(time.Microsecond)

	initial := guardrail.GuardrailConfig{ID: id, Enabled: true, SystemRules: guardrail.DefaultSystemRulesConfig(), CreatedAt: now, UpdatedAt: now}
	if err := repo.UpsertConfig(ctx, initial); err != nil {
		t.Fatalf("unexpected insert error: %v", err)
	}

	updated := initial
	updated.Enabled = false
	updated.UpdatedAt = now.Add(time.Minute)
	if err := repo.UpsertConfig(ctx, updated); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	got, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got.Enabled {
		t.Fatalf("expected config to be disabled after update")
	}
	if got.ID != id {
		t.Fatalf("expected same singleton ID to be preserved, got %s", got.ID)
	}
}

func TestRepository_GetConfig_NotFoundWhenEmpty(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)

	_, err := repo.GetConfig(context.Background())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ── Custom rules ─────────────────────────────────────────────────────

func TestRepository_CreateFindUpdateDeleteRule(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)
	ctx := context.Background()

	cfg, _ := json.Marshal(guardrail.BlockedTermsConfig{Terms: []string{"badword"}, MatchType: "contains"})
	rule := guardrail.GuardrailRule{
		ID:        identity.NewGuardrailRuleID(),
		Name:      "block-badword",
		Type:      guardrail.RuleTypeBlockedTerms,
		Config:    cfg,
		Priority:  10,
		Enabled:   true,
		Action:    guardrail.ActionBlock,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	if err := repo.CreateRule(ctx, rule); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	found, err := repo.FindRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if found.Name != "block-badword" || found.Type != guardrail.RuleTypeBlockedTerms {
		t.Fatalf("unexpected rule: %+v", found)
	}

	list, err := repo.ListRules(ctx)
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(list) != 1 || list[0].ID != rule.ID {
		t.Fatalf("expected single rule in list, got %+v", list)
	}

	rule.Name = "block-badword-updated"
	rule.Enabled = false
	rule.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.UpdateRule(ctx, rule); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	updated, err := repo.FindRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if updated.Name != "block-badword-updated" || updated.Enabled {
		t.Fatalf("unexpected updated rule: %+v", updated)
	}

	if err := repo.DeleteRule(ctx, rule.ID); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := repo.FindRule(ctx, rule.ID); !isNotFound(err) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestRepository_FindRule_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)

	_, err := repo.FindRule(context.Background(), identity.NewGuardrailRuleID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_UpdateRule_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)

	rule := guardrail.GuardrailRule{
		ID:     identity.NewGuardrailRuleID(),
		Name:   "ghost",
		Config: json.RawMessage(`{}`),
		Action: guardrail.ActionWarn,
	}
	if err := repo.UpdateRule(context.Background(), rule); !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_DeleteRule_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)

	err := repo.DeleteRule(context.Background(), identity.NewGuardrailRuleID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_ListRules_OrderedByPriority(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)
	ctx := context.Background()

	low := mustCreateRule(t, repo, "low-priority", 100)
	high := mustCreateRule(t, repo, "high-priority", 1)

	list, err := repo.ListRules(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(list))
	}
	if list[0].ID != high || list[1].ID != low {
		t.Fatalf("expected rules ordered by priority ascending, got %+v", list)
	}
}

// ── Violations ───────────────────────────────────────────────────────

func TestRepository_LogAndListViolations(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)
	ctx := context.Background()

	v := guardrail.GuardrailViolation{
		ID:          identity.NewGuardrailViolationID(),
		RuleID:      "system:pii",
		RuleName:    "PII Detection",
		Category:    "pii",
		ActionTaken: "redacted",
		Model:       "gpt-4o",
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := repo.LogViolation(ctx, v); err != nil {
		t.Fatalf("unexpected log error: %v", err)
	}

	page, err := repo.ListViolations(ctx, query.Pagination{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].RuleName != "PII Detection" {
		t.Fatalf("unexpected violations: %+v", page.Items)
	}
	if page.Page.Total != 1 {
		t.Fatalf("expected total 1, got %d", page.Page.Total)
	}
}

func TestRepository_ListViolations_EmptyOptionalFields(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := guardrailpg.New(db)
	ctx := context.Background()

	v := guardrail.GuardrailViolation{
		ID:          identity.NewGuardrailViolationID(),
		RuleID:      "custom:1",
		RuleName:    "Blocked Terms",
		Category:    "blocked_terms",
		ActionTaken: "blocked",
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := repo.LogViolation(ctx, v); err != nil {
		t.Fatalf("unexpected error logging violation with empty optional fields: %v", err)
	}

	page, err := repo.ListViolations(ctx, query.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].MatchedPattern != "" || page.Items[0].Model != "" {
		t.Fatalf("unexpected violation: %+v", page.Items)
	}
}

// ── helpers ──────────────────────────────────────────────────────────

func isNotFound(err error) bool {
	var appErr *errx.Error
	return errx.As(err, &appErr) && appErr.Type == errx.TypeNotFound
}

func mustCreateRule(t *testing.T, repo *guardrailpg.Repository, name string, priority int) identity.GuardrailRuleID {
	t.Helper()
	rule := guardrail.GuardrailRule{
		ID:        identity.NewGuardrailRuleID(),
		Name:      name,
		Type:      guardrail.RuleTypeCustomRegex,
		Config:    json.RawMessage(`{"pattern":"foo"}`),
		Priority:  priority,
		Enabled:   true,
		Action:    guardrail.ActionWarn,
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	if err := repo.CreateRule(context.Background(), rule); err != nil {
		t.Fatalf("failed to create rule fixture: %v", err)
	}
	return rule.ID
}
