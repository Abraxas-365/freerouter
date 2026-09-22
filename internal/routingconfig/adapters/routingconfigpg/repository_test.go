package routingconfigpg_test

import (
	"context"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/routingconfig"
	"github.com/Abraxas-365/freerouter/internal/routingconfig/adapters/routingconfigpg"
	"github.com/Abraxas-365/freerouter/internal/testutil"
)

func TestRepository_CreateFindUpdateDelete(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := routingconfig.RoutingConfig{
		ID:        identity.NewRoutingConfigID(),
		SubjectID: "user-123",
		Strategy:  routingconfig.StrategyLowestLatency,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	found, err := repo.Find(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if found.SubjectID != cfg.SubjectID || found.Strategy != cfg.Strategy {
		t.Fatalf("unexpected config: %+v", found)
	}

	bySubject, err := repo.FindBySubject(ctx, "user-123")
	if err != nil {
		t.Fatalf("unexpected find by subject error: %v", err)
	}
	if bySubject.ID != cfg.ID {
		t.Fatalf("expected same config by subject lookup, got %+v", bySubject)
	}

	updated := found
	updated.Strategy = routingconfig.StrategyRoundRobin
	updated.UpdatedAt = now.Add(time.Minute)
	if err := repo.Update(ctx, updated); err != nil {
		t.Fatalf("unexpected update error: %v", err)
	}

	after, err := repo.Find(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if after.Strategy != routingconfig.StrategyRoundRobin {
		t.Fatalf("expected updated strategy, got %s", after.Strategy)
	}

	if err := repo.Delete(ctx, cfg.ID); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
	if _, err := repo.Find(ctx, cfg.ID); !isNotFound(err) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestRepository_Find_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)

	_, err := repo.Find(context.Background(), identity.NewRoutingConfigID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_FindBySubject_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)

	_, err := repo.FindBySubject(context.Background(), "missing-subject")
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_Update_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)

	cfg := routingconfig.RoutingConfig{ID: identity.NewRoutingConfigID(), SubjectID: "x", Strategy: routingconfig.StrategyCheapest}
	err := repo.Update(context.Background(), cfg)
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)

	err := repo.Delete(context.Background(), identity.NewRoutingConfigID())
	if !isNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestRepository_Create_DuplicateSubject_Conflicts(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := routingconfig.RoutingConfig{
		ID: identity.NewRoutingConfigID(), SubjectID: "dup-subject", Strategy: routingconfig.StrategyCheapest,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatalf("unexpected first create error: %v", err)
	}

	dup := cfg
	dup.ID = identity.NewRoutingConfigID()
	err := repo.Create(ctx, dup)
	var appErr *errx.Error
	if !errx.As(err, &appErr) || appErr.Type != errx.TypeConflict {
		t.Fatalf("expected conflict error for duplicate subject, got %v", err)
	}
}

func TestRepository_List_FilterBySubject(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)
	ctx := context.Background()

	mustCreate(t, repo, "subject-a", routingconfig.StrategyCheapest)
	mustCreate(t, repo, "subject-b", routingconfig.StrategyRoundRobin)

	subj := "subject-a"
	page, err := repo.List(ctx, routingconfig.Filter{SubjectID: &subj}, query.Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].SubjectID != "subject-a" {
		t.Fatalf("expected only subject-a, got %+v", page.Items)
	}
}

func TestRepository_List_Pagination(t *testing.T) {
	db := testutil.PostgresDB(t)
	repo := routingconfigpg.New(db)
	ctx := context.Background()

	subjects := []string{"pag-subject-1", "pag-subject-2", "pag-subject-3"}
	for _, s := range subjects {
		mustCreate(t, repo, s, routingconfig.StrategyCheapest)
	}

	page, err := repo.List(ctx, routingconfig.Filter{}, query.Pagination{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 2 || page.Page.Total != 3 {
		t.Fatalf("expected 2 items of 3 total, got %d items, total %d", len(page.Items), page.Page.Total)
	}
}

// ── helpers ──────────────────────────────────────────────────────────

func isNotFound(err error) bool {
	var appErr *errx.Error
	return errx.As(err, &appErr) && appErr.Type == errx.TypeNotFound
}

func mustCreate(t *testing.T, repo *routingconfigpg.Repository, subjectID string, strategy routingconfig.Strategy) identity.RoutingConfigID {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	cfg := routingconfig.RoutingConfig{
		ID:        identity.NewRoutingConfigID(),
		SubjectID: subjectID,
		Strategy:  strategy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), cfg); err != nil {
		t.Fatalf("failed to create routing config fixture: %v", err)
	}
	return cfg.ID
}
