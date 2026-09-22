package ratelimitredis

import (
	"context"
	"fmt"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/ratelimit"
	"github.com/redis/go-redis/v9"
)

// Limiter implements ratelimit.Limiter using Redis.
// RPM uses a sorted-set sliding window; concurrency uses INCR/DECR.
type Limiter struct{ rdb *redis.Client }

var _ ratelimit.Limiter = (*Limiter)(nil)

// New creates a Redis-backed rate limiter.
func New(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// Check enforces RPM (sliding window) and acquires a concurrency slot.
// On success the caller MUST call Release when the request completes.
func (l *Limiter) Check(ctx context.Context, subjectID string, rpm, maxConcurrent int) (*ratelimit.RateLimitResult, error) {
	// ── RPM check (sorted-set sliding window) ──────────────────────
	if rpm > 0 {
		result, err := l.checkRPM(ctx, subjectID, rpm)
		if err != nil {
			return nil, err
		}
		if !result.Allowed {
			return result, nil
		}
	}

	// ── Concurrency check ──────────────────────────────────────────
	if maxConcurrent > 0 {
		acquired, err := l.acquireConcurrency(ctx, subjectID, maxConcurrent)
		if err != nil {
			return nil, err
		}
		if !acquired {
			return &ratelimit.RateLimitResult{
				Allowed:    false,
				Remaining:  0,
				Limit:      maxConcurrent,
				RetryAfter: time.Second,
			}, nil
		}
	}

	remaining := 0
	limit := rpm
	if rpm > 0 {
		// Approximate remaining after admission
		remaining = rpm - 1
	}

	return &ratelimit.RateLimitResult{
		Allowed:   true,
		Remaining: remaining,
		Limit:     limit,
	}, nil
}

// Release frees the concurrency slot for the subject.
func (l *Limiter) Release(ctx context.Context, subjectID string) {
	key := "ratelimit:concurrent:" + subjectID
	count, err := l.rdb.Decr(ctx, key).Result()
	if err == nil && count <= 0 {
		_ = l.rdb.Del(ctx, key).Err()
	}
}

// ── internal ────────────────────────────────────────────────────────

func (l *Limiter) checkRPM(ctx context.Context, subjectID string, rpm int) (*ratelimit.RateLimitResult, error) {
	key := "ratelimit:rpm:" + subjectID
	now := time.Now()
	windowStart := now.Add(-time.Minute).UnixMilli()

	// Remove expired entries and count current ones
	pipe := l.rdb.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart))
	countCmd := pipe.ZCard(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errx.Wrap(err, "redis rpm check failed", errx.TypeExternal)
	}

	count := countCmd.Val()
	if count >= int64(rpm) {
		return &ratelimit.RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			Limit:      rpm,
			RetryAfter: time.Second,
		}, nil
	}

	// Add this request to the window
	member := fmt.Sprintf("%d-%d", now.UnixNano(), count)
	pipe = l.rdb.TxPipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMilli()), Member: member})
	pipe.Expire(ctx, key, time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errx.Wrap(err, "redis rpm record failed", errx.TypeExternal)
	}

	return &ratelimit.RateLimitResult{
		Allowed:   true,
		Remaining: rpm - int(count) - 1,
		Limit:     rpm,
	}, nil
}

func (l *Limiter) acquireConcurrency(ctx context.Context, subjectID string, maxConcurrent int) (bool, error) {
	key := "ratelimit:concurrent:" + subjectID
	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, errx.Wrap(err, "redis concurrency acquire failed", errx.TypeExternal)
	}
	// Set TTL on first increment as a safety net
	if count == 1 {
		_ = l.rdb.Expire(ctx, key, 5*time.Minute).Err()
	}
	if count > int64(maxConcurrent) {
		_ = l.rdb.Decr(ctx, key).Err()
		return false, nil
	}
	return true, nil
}
