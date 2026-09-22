package usagepg_test

import (
	"context"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/testutil"
	"github.com/Abraxas-365/freerouter/internal/usage"
	"github.com/Abraxas-365/freerouter/internal/usage/adapters/usagepg"
)

// ── Usage logs ──────────────────────────────────────────────────────

func TestRepository_CreateFindUsageLog(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)
	ctx := context.Background()

	log := newUsageLogFixture()
	if err := repo.Create(ctx, log); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	got, err := repo.Find(ctx, log.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if got.ID != log.ID || got.RequestedModel != log.RequestedModel {
		t.Fatalf("unexpected log: %+v", got)
	}
}

func TestRepository_Find_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)

	_, err := repo.Find(context.Background(), identity.NewUsageLogID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ── Retention config (global singleton) ────────────────────────────

func TestRepository_UpsertRetention_InsertsWhenMissing(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := usage.RetentionConfig{
		ID:                 identity.NewRetentionConfigID(),
		RetentionDays:      30,
		RetainMessages:     true,
		RetainResponseBody: false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := repo.UpsertRetention(ctx, cfg); err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	got, err := repo.GetRetention(ctx)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got.ID != cfg.ID || got.RetentionDays != 30 || !got.RetainMessages || got.RetainResponseBody {
		t.Fatalf("unexpected config: %+v", got)
	}
}

func TestRepository_UpsertRetention_UpdatesExistingRow(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)
	ctx := context.Background()

	id := identity.NewRetentionConfigID()
	now := time.Now().UTC().Truncate(time.Microsecond)

	initial := usage.RetentionConfig{
		ID: id, RetentionDays: 90, RetainMessages: true, RetainResponseBody: true,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.UpsertRetention(ctx, initial); err != nil {
		t.Fatalf("unexpected insert error: %v", err)
	}

	updated := initial
	updated.RetentionDays = 7
	updated.RetainResponseBody = false
	updated.UpdatedAt = now.Add(time.Minute)
	if err := repo.UpsertRetention(ctx, updated); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	got, err := repo.GetRetention(ctx)
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got.ID != id {
		t.Fatalf("expected same singleton ID to be preserved, got %s", got.ID)
	}
	if got.RetentionDays != 7 || got.RetainResponseBody {
		t.Fatalf("expected updated config, got %+v", got)
	}
}

func TestRepository_GetRetention_NotFoundWhenEmpty(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)

	_, err := repo.GetRetention(context.Background())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_DeleteRetention(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := usage.RetentionConfig{
		ID: identity.NewRetentionConfigID(), RetentionDays: 30,
		RetainMessages: true, RetainResponseBody: true,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.UpsertRetention(ctx, cfg); err != nil {
		t.Fatalf("unexpected upsert error: %v", err)
	}

	if err := repo.DeleteRetention(ctx); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	_, err := repo.GetRetention(ctx)
	if !isNotFound(err) {
		t.Fatalf("expected not-found error after delete, got %v", err)
	}
}

// ── Purge ───────────────────────────────────────────────────────────

func TestRepository_PurgeOlderThan(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)
	ctx := context.Background()

	oldLog := newUsageLogFixture()
	if err := repo.Create(ctx, oldLog); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	// Backdate the row directly since Create always uses the fixture's CreatedAt.
	if _, err := db.ExecContext(ctx,
		`UPDATE usage_logs SET created_at = now() - interval '100 days' WHERE id = $1`, oldLog.ID); err != nil {
		t.Fatalf("failed to backdate fixture: %v", err)
	}

	recentLog := newUsageLogFixture()
	if err := repo.Create(ctx, recentLog); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	n, err := repo.PurgeOlderThan(ctx, 90)
	if err != nil {
		t.Fatalf("unexpected purge error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row purged, got %d", n)
	}

	if _, err := repo.Find(ctx, oldLog.ID); !isNotFound(err) {
		t.Fatalf("expected old log to be purged, got err=%v", err)
	}
	if _, err := repo.Find(ctx, recentLog.ID); err != nil {
		t.Fatalf("expected recent log to survive purge, got err=%v", err)
	}
}

func TestRepository_PurgeOlderThan_NoMatches(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := usagepg.New(db)
	ctx := context.Background()

	log := newUsageLogFixture()
	if err := repo.Create(ctx, log); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	n, err := repo.PurgeOlderThan(ctx, 90)
	if err != nil {
		t.Fatalf("unexpected purge error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 rows purged, got %d", n)
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func newUsageLogFixture() usage.UsageLog {
	return usage.UsageLog{
		ID:               identity.NewUsageLogID(),
		KeyID:            identity.NewProviderKeyID(),
		RequestedModel:   "gpt-test",
		UsedModel:        "gpt-test-real",
		ProviderID:       identity.NewProviderID(),
		MappingID:        identity.NewMappingID(),
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		InputCost:        0.001,
		OutputCost:       0.002,
		TotalCost:        0.003,
		DurationMs:       120,
		StatusCode:       200,
		FinishReason:     "stop",
		CreatedAt:        time.Now().UTC().Truncate(time.Microsecond),
	}
}

func isNotFound(err error) bool {
	var appErr *errx.Error
	return errx.As(err, &appErr) && appErr.Type == errx.TypeNotFound
}
