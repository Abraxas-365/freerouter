package gateway

import (
	"context"
	"testing"

	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return rdb, func() {
		rdb.Close()
		mr.Close()
	}
}

// mockConfigRepo implements RateLimitConfigRepository in memory for testing.
type mockConfigRepo struct {
	configs map[string]*RateLimitConfig
}

func newMockConfigRepo() *mockConfigRepo {
	return &mockConfigRepo{configs: make(map[string]*RateLimitConfig)}
}

func (m *mockConfigRepo) GetByTenantID(_ context.Context, tenantID kernel.TenantID) (*RateLimitConfig, error) {
	cfg, ok := m.configs[tenantID.String()]
	if !ok {
		return nil, nil
	}
	return cfg, nil
}

func (m *mockConfigRepo) Upsert(_ context.Context, cfg *RateLimitConfig) (*RateLimitConfig, error) {
	m.configs[cfg.TenantID.String()] = cfg
	return cfg, nil
}

func (m *mockConfigRepo) Delete(_ context.Context, tenantID kernel.TenantID) error {
	delete(m.configs, tenantID.String())
	return nil
}

func TestRateLimiter_RPM_AllowsUnderLimit(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 10, MaxConcurrent: 0}, nil)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		result, err := rl.CheckRPM(ctx, "tenant-1")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
		if result.Remaining < 0 {
			t.Fatalf("remaining should be >= 0, got %d", result.Remaining)
		}
	}
}

func TestRateLimiter_RPM_BlocksOverLimit(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 5, MaxConcurrent: 0}, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, _ := rl.CheckRPM(ctx, "tenant-1")
		if !result.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	result, _ := rl.CheckRPM(ctx, "tenant-1")
	if result.Allowed {
		t.Fatal("6th request should be blocked")
	}
	if result.RetryAfter <= 0 {
		t.Fatal("RetryAfter should be > 0")
	}
}

func TestRateLimiter_RPM_TenantsIndependent(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 3, MaxConcurrent: 0}, nil)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		rl.CheckRPM(ctx, "tenant-1")
	}

	result, _ := rl.CheckRPM(ctx, "tenant-2")
	if !result.Allowed {
		t.Fatal("tenant-2 should not be affected by tenant-1's limits")
	}
}

func TestRateLimiter_Concurrency_AcquireRelease(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 0, MaxConcurrent: 2}, nil)
	ctx := context.Background()

	ok1, _ := rl.AcquireConcurrency(ctx, "tenant-1")
	ok2, _ := rl.AcquireConcurrency(ctx, "tenant-1")
	if !ok1 || !ok2 {
		t.Fatal("should acquire both slots")
	}

	ok3, _ := rl.AcquireConcurrency(ctx, "tenant-1")
	if ok3 {
		t.Fatal("3rd slot should be denied")
	}

	rl.ReleaseConcurrency(ctx, "tenant-1")

	ok4, _ := rl.AcquireConcurrency(ctx, "tenant-1")
	if !ok4 {
		t.Fatal("should be able to acquire after release")
	}
}

func TestRateLimiter_Concurrency_TenantsIndependent(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 0, MaxConcurrent: 1}, nil)
	ctx := context.Background()

	ok1, _ := rl.AcquireConcurrency(ctx, "tenant-1")
	if !ok1 {
		t.Fatal("tenant-1 should acquire")
	}

	ok2, _ := rl.AcquireConcurrency(ctx, "tenant-2")
	if !ok2 {
		t.Fatal("tenant-2 should be independent")
	}
}

func TestRateLimiter_Check_BothLimits(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 10, MaxConcurrent: 2}, nil)
	ctx := context.Background()

	result, _ := rl.Check(ctx, "tenant-1")
	if !result.Allowed {
		t.Fatal("first check should be allowed")
	}

	result, _ = rl.Check(ctx, "tenant-1")
	if !result.Allowed {
		t.Fatal("second check should be allowed")
	}

	result, _ = rl.Check(ctx, "tenant-1")
	if result.Allowed {
		t.Fatal("third check should be denied (concurrency limit)")
	}

	rl.Release(ctx, "tenant-1")

	result, _ = rl.Check(ctx, "tenant-1")
	if !result.Allowed {
		t.Fatal("should pass after release")
	}
}

func TestRateLimiter_NilRedis_FailsOpen(t *testing.T) {
	rl := NewRateLimiter(nil, RateLimitConfig{RPM: 1, MaxConcurrent: 1}, nil)
	ctx := context.Background()

	result, err := rl.CheckRPM(ctx, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed {
		t.Fatal("nil Redis should fail open")
	}

	ok, err := rl.AcquireConcurrency(ctx, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("nil Redis should fail open for concurrency")
	}
}

func TestRateLimiter_ZeroLimits_Unlimited(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 0, MaxConcurrent: 0}, nil)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		result, _ := rl.CheckRPM(ctx, "tenant-1")
		if !result.Allowed {
			t.Fatal("zero RPM means unlimited")
		}
	}

	for i := 0; i < 100; i++ {
		ok, _ := rl.AcquireConcurrency(ctx, "tenant-1")
		if !ok {
			t.Fatal("zero concurrency means unlimited")
		}
	}
}

// ============================================================================
// Per-Tenant Config Tests
// ============================================================================

func TestRateLimiter_PerTenantConfig_OverridesDefault(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	repo := newMockConfigRepo()
	repo.configs["tenant-1"] = &RateLimitConfig{
		TenantID: kernel.NewTenantID("tenant-1"),
		RPM:      3,
	}

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 100, MaxConcurrent: 0}, repo)
	ctx := context.Background()

	// tenant-1 should hit limit at 3
	for i := 0; i < 3; i++ {
		result, _ := rl.CheckRPM(ctx, "tenant-1")
		if !result.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	result, _ := rl.CheckRPM(ctx, "tenant-1")
	if result.Allowed {
		t.Fatal("4th request for tenant-1 should be blocked (custom limit = 3)")
	}

	// tenant-2 should use default (100)
	for i := 0; i < 10; i++ {
		result, _ := rl.CheckRPM(ctx, "tenant-2")
		if !result.Allowed {
			t.Fatal("tenant-2 should use default limit and allow request")
		}
	}
}

func TestRateLimiter_PerTenantConfig_ConcurrencyOverride(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	repo := newMockConfigRepo()
	repo.configs["tenant-1"] = &RateLimitConfig{
		TenantID:      kernel.NewTenantID("tenant-1"),
		RPM:           0,
		MaxConcurrent: 1,
	}

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 0, MaxConcurrent: 10}, repo)
	ctx := context.Background()

	ok, _ := rl.AcquireConcurrency(ctx, "tenant-1")
	if !ok {
		t.Fatal("first slot should be acquired")
	}

	ok, _ = rl.AcquireConcurrency(ctx, "tenant-1")
	if ok {
		t.Fatal("2nd slot should be denied (custom limit = 1)")
	}
}

func TestRateLimiter_InvalidateCache(t *testing.T) {
	rdb, cleanup := newTestRedis(t)
	defer cleanup()

	repo := newMockConfigRepo()
	repo.configs["tenant-1"] = &RateLimitConfig{
		TenantID: kernel.NewTenantID("tenant-1"),
		RPM:      2,
	}

	rl := NewRateLimiter(rdb, RateLimitConfig{RPM: 100, MaxConcurrent: 0}, repo)
	ctx := context.Background()

	// Warm the cache
	rl.CheckRPM(ctx, "tenant-1")
	rl.CheckRPM(ctx, "tenant-1")
	result, _ := rl.CheckRPM(ctx, "tenant-1")
	if result.Allowed {
		t.Fatal("should be blocked at RPM=2")
	}

	// Update the config and invalidate cache
	repo.configs["tenant-1"].RPM = 100
	rl.InvalidateCache("tenant-1")

	// tenant-2 should use default (100)
	result, _ = rl.CheckRPM(ctx, "tenant-2")
	if !result.Allowed {
		t.Fatal("tenant-2 should be allowed (default 100)")
	}
}
