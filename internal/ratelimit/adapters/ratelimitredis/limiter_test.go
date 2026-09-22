package ratelimitredis_test

import (
	"context"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ratelimit/adapters/ratelimitredis"
	"github.com/Abraxas-365/freerouter/internal/testutil"
	"github.com/redis/go-redis/v9"
)

func TestLimiter_Check_AllowsWithinRPMLimit(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-rpm-allow"

	for i := 0; i < 3; i++ {
		result, err := limiter.Check(ctx, subject, 5, 0)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed, got %+v", i, result)
		}
		if result.Limit != 5 {
			t.Fatalf("expected limit 5, got %d", result.Limit)
		}
	}
}

func TestLimiter_Check_BlocksOverRPMLimit(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-rpm-block"

	for i := 0; i < 2; i++ {
		result, err := limiter.Check(ctx, subject, 2, 0)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed, got %+v", i, result)
		}
	}

	// Third request exceeds the RPM=2 window.
	result, err := limiter.Check(ctx, subject, 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected third request to be rate-limited, got %+v", result)
	}
	if result.RetryAfter <= 0 {
		t.Fatalf("expected positive RetryAfter, got %v", result.RetryAfter)
	}
}

func TestLimiter_Check_DifferentSubjectsHaveIndependentWindows(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()

	// Exhaust subject A's window.
	for i := 0; i < 2; i++ {
		if _, err := limiter.Check(ctx, "subject-a", 2, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	blocked, err := limiter.Check(ctx, "subject-a", 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked.Allowed {
		t.Fatalf("expected subject-a to be blocked")
	}

	// Subject B should be unaffected.
	allowed, err := limiter.Check(ctx, "subject-b", 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed.Allowed {
		t.Fatalf("expected subject-b to be allowed independently, got %+v", allowed)
	}
}

func TestLimiter_Check_ZeroRPMSkipsRateCheck(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-no-rpm-limit"

	for i := 0; i < 10; i++ {
		result, err := limiter.Check(ctx, subject, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i, err)
		}
		if !result.Allowed {
			t.Fatalf("expected unlimited RPM to always allow, got %+v on request %d", result, i)
		}
	}
}

// ── Concurrency limiting ────────────────────────────────────────────

func TestLimiter_Check_ConcurrencyLimitEnforced(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-concurrency"

	first, err := limiter.Check(ctx, subject, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !first.Allowed {
		t.Fatalf("expected first concurrent slot to be allowed")
	}

	second, err := limiter.Check(ctx, subject, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Allowed {
		t.Fatalf("expected second concurrent request to be rejected while slot is held")
	}

	// Release the slot; a subsequent request should now be admitted.
	limiter.Release(ctx, subject)

	third, err := limiter.Check(ctx, subject, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !third.Allowed {
		t.Fatalf("expected slot to be available after release, got %+v", third)
	}
}

func TestLimiter_Release_DoesNotUnderflowBelowZero(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-release-underflow"

	// Releasing a slot that was never acquired should not error or panic,
	// and must not leave a negative counter that breaks future checks.
	limiter.Release(ctx, subject)

	result, err := limiter.Check(ctx, subject, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected slot to be available after spurious release, got %+v", result)
	}
}

func TestLimiter_Check_CombinedRPMAndConcurrency(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-combined"

	// RPM limit of 1 should block the second request even though a
	// concurrency slot would otherwise be available.
	first, err := limiter.Check(ctx, subject, 1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !first.Allowed {
		t.Fatalf("expected first request admitted, got %+v", first)
	}

	second, err := limiter.Check(ctx, subject, 1, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Allowed {
		t.Fatalf("expected RPM limit to block second request, got %+v", second)
	}
}

func TestLimiter_Check_RPMWindowSlidesOverTime(t *testing.T) {
	rdb := testutil.RedisClient(t)
	limiter := ratelimitredis.New(rdb)
	ctx := context.Background()
	subject := "subject-sliding-window"

	// Manually seed an old sorted-set entry outside the 1-minute window so
	// we can assert it gets pruned by ZRemRangeByScore.
	key := "ratelimit:rpm:" + subject
	oldScore := float64(time.Now().Add(-2 * time.Minute).UnixMilli())
	if err := rdb.ZAdd(ctx, key, redis.Z{Score: oldScore, Member: "stale-member"}).Err(); err != nil {
		t.Fatalf("failed to seed stale entry: %v", err)
	}

	result, err := limiter.Check(ctx, subject, 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatalf("expected request to be allowed since stale entry should be pruned, got %+v", result)
	}

	count, err := rdb.ZCard(ctx, key).Result()
	if err != nil {
		t.Fatalf("unexpected error counting set: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 member (the new one) after pruning, got %d", count)
	}
}
