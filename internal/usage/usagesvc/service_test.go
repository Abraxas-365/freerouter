package usagesvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/usage"
)

// fakeRepo is an in-memory usage.Repository for exercising Service logic
// without a real database.
type fakeRepo struct {
	mu        sync.Mutex
	retention *usage.RetentionConfig
	purgeDays []int
	purgeN    int64
	purgeErr  error
}

var _ usage.Repository = (*fakeRepo)(nil)

func (f *fakeRepo) Create(ctx context.Context, log usage.UsageLog) error { return nil }
func (f *fakeRepo) Find(ctx context.Context, id identity.UsageLogID) (usage.UsageLog, error) {
	return usage.UsageLog{}, errx.NotFound("not found")
}
func (f *fakeRepo) List(ctx context.Context, filter usage.Filter, page query.Pagination) (query.Paginated[usage.UsageLog], error) {
	return query.Paginated[usage.UsageLog]{}, nil
}
func (f *fakeRepo) GetSummary(ctx context.Context, from, to *time.Time) (*usage.Summary, error) {
	return &usage.Summary{}, nil
}
func (f *fakeRepo) GetSummaryByModel(ctx context.Context, from, to *time.Time) ([]usage.ModelSummary, error) {
	return nil, nil
}

func (f *fakeRepo) GetRetention(ctx context.Context) (usage.RetentionConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.retention == nil {
		return usage.RetentionConfig{}, errx.NotFound("retention config not found")
	}
	return *f.retention, nil
}

func (f *fakeRepo) UpsertRetention(ctx context.Context, cfg usage.RetentionConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retention = &cfg
	return nil
}

func (f *fakeRepo) DeleteRetention(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retention = nil
	return nil
}

func (f *fakeRepo) PurgeOlderThan(ctx context.Context, days int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.purgeDays = append(f.purgeDays, days)
	return f.purgeN, f.purgeErr
}

// ── UpsertRetention ─────────────────────────────────────────────────

func TestService_UpsertRetention_CreatesWithDefaultsWhenMissing(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 10)
	defer svc.Close()

	days := 30
	cfg, err := svc.UpsertRetention(context.Background(), usage.UpsertRetention{RetentionDays: &days})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("expected retention_days 30, got %d", cfg.RetentionDays)
	}
	if !cfg.RetainMessages || !cfg.RetainResponseBody {
		t.Errorf("expected defaults for unset fields, got %+v", cfg)
	}
}

func TestService_UpsertRetention_MergesWithExisting(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 10)
	defer svc.Close()

	ctx := context.Background()
	days := 30
	if _, err := svc.UpsertRetention(ctx, usage.UpsertRetention{RetentionDays: &days}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retainMessages := false
	cfg, err := svc.UpsertRetention(ctx, usage.UpsertRetention{RetainMessages: &retainMessages})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RetentionDays != 30 {
		t.Errorf("expected retention_days to be preserved at 30, got %d", cfg.RetentionDays)
	}
	if cfg.RetainMessages {
		t.Errorf("expected retain_messages to be updated to false")
	}
}

func TestService_UpsertRetention_RejectsNegativeDays(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 10)
	defer svc.Close()

	days := -1
	_, err := svc.UpsertRetention(context.Background(), usage.UpsertRetention{RetentionDays: &days})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestService_DeleteRetention(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 10)
	defer svc.Close()

	ctx := context.Background()
	days := 30
	if _, err := svc.UpsertRetention(ctx, usage.UpsertRetention{RetentionDays: &days}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.DeleteRetention(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.GetRetention(ctx); err == nil {
		t.Fatal("expected not-found error after delete")
	}
}

// ── runPurge ────────────────────────────────────────────────────────

func TestService_RunPurge_NoOpWhenNoConfig(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 10)
	defer svc.Close()

	svc.runPurge()

	if len(repo.purgeDays) != 0 {
		t.Errorf("expected no purge call when no retention config exists, got %v", repo.purgeDays)
	}
}

func TestService_RunPurge_NoOpWhenRetainForever(t *testing.T) {
	repo := &fakeRepo{retention: &usage.RetentionConfig{RetentionDays: 0}}
	svc := New(repo, 10)
	defer svc.Close()

	svc.runPurge()

	if len(repo.purgeDays) != 0 {
		t.Errorf("expected no purge call when retention_days is 0, got %v", repo.purgeDays)
	}
}

func TestService_RunPurge_PurgesWhenConfigured(t *testing.T) {
	repo := &fakeRepo{retention: &usage.RetentionConfig{RetentionDays: 14}, purgeN: 5}
	svc := New(repo, 10)
	defer svc.Close()

	svc.runPurge()

	if len(repo.purgeDays) != 1 || repo.purgeDays[0] != 14 {
		t.Errorf("expected purge called with 14 days, got %v", repo.purgeDays)
	}
}

// ── purge worker lifecycle ─────────────────────────────────────────

func TestService_StartStopPurgeWorker(t *testing.T) {
	repo := &fakeRepo{retention: &usage.RetentionConfig{RetentionDays: 1}, purgeN: 1}
	svc := New(repo, 10)
	defer svc.Close()

	svc.StartPurgeWorker(10 * time.Millisecond)
	time.Sleep(35 * time.Millisecond)
	svc.StopPurgeWorker()

	repo.mu.Lock()
	calls := len(repo.purgeDays)
	repo.mu.Unlock()

	if calls == 0 {
		t.Error("expected at least one purge call from the ticking worker")
	}
}
