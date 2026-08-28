package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig holds per-tenant rate limit settings.
type RateLimitConfig struct {
	TenantID      kernel.TenantID `db:"tenant_id" json:"tenant_id"`
	RPM           int             `db:"rpm" json:"rpm"`
	MaxConcurrent int             `db:"max_concurrent" json:"max_concurrent"`
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RPM:           60,
		MaxConcurrent: 10,
	}
}

// RateLimitConfigRepository loads per-tenant rate limit config from the DB.
type RateLimitConfigRepository interface {
	GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*RateLimitConfig, error)
	Upsert(ctx context.Context, cfg *RateLimitConfig) (*RateLimitConfig, error)
	Delete(ctx context.Context, tenantID kernel.TenantID) error
}

// UpsertRateLimitRequest is the DTO for creating/updating rate limit config.
type UpsertRateLimitRequest struct {
	RPM           *int `json:"rpm"`
	MaxConcurrent *int `json:"max_concurrent"`
}

// RateLimitResult contains the outcome of a rate limit check.
type RateLimitResult struct {
	Allowed    bool
	Remaining  int
	Limit      int
	RetryAfter time.Duration // >0 if not allowed
}

// RateLimiter enforces per-tenant rate limits using Redis.
// Uses sliding window log for RPM and a Redis counter for concurrency.
type RateLimiter struct {
	rdb           *redis.Client
	defaultConfig RateLimitConfig
	configRepo    RateLimitConfigRepository // nil = use defaults for everyone
	configCache   sync.Map                  // tenantID -> *cachedConfig
}

type cachedConfig struct {
	config    RateLimitConfig
	expiresAt time.Time
}

const configCacheTTL = 30 * time.Second

func NewRateLimiter(rdb *redis.Client, defaultConfig RateLimitConfig, configRepo RateLimitConfigRepository) *RateLimiter {
	return &RateLimiter{
		rdb:           rdb,
		defaultConfig: defaultConfig,
		configRepo:    configRepo,
	}
}

// getConfig returns the rate limit config for a tenant, with in-memory caching.
func (rl *RateLimiter) getConfig(ctx context.Context, tenantID string) RateLimitConfig {
	// Check in-memory cache first
	if cached, ok := rl.configCache.Load(tenantID); ok {
		cc := cached.(*cachedConfig)
		if time.Now().Before(cc.expiresAt) {
			return cc.config
		}
	}

	// Load from DB
	if rl.configRepo != nil {
		cfg, err := rl.configRepo.GetByTenantID(ctx, kernel.NewTenantID(tenantID))
		if err == nil && cfg != nil {
			rl.configCache.Store(tenantID, &cachedConfig{
				config:    *cfg,
				expiresAt: time.Now().Add(configCacheTTL),
			})
			return *cfg
		}
	}

	// Cache the default too so we don't hit DB on every request for unconfigured tenants
	rl.configCache.Store(tenantID, &cachedConfig{
		config:    rl.defaultConfig,
		expiresAt: time.Now().Add(configCacheTTL),
	})
	return rl.defaultConfig
}

// InvalidateCache removes the cached config for a tenant (call after upsert/delete).
func (rl *RateLimiter) InvalidateCache(tenantID string) {
	rl.configCache.Delete(tenantID)
}

// CheckRPM checks whether the tenant is within their RPM limit.
func (rl *RateLimiter) CheckRPM(ctx context.Context, tenantID string) (*RateLimitResult, error) {
	cfg := rl.getConfig(ctx, tenantID)
	if rl.rdb == nil || cfg.RPM <= 0 {
		return &RateLimitResult{Allowed: true, Remaining: -1}, nil
	}

	key := fmt.Sprintf("rl:rpm:%s", tenantID)
	now := time.Now()
	windowStart := now.Add(-time.Minute)
	member := fmt.Sprintf("%d:%d", now.UnixNano(), now.UnixNano()%1000000)

	pipe := rl.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: member})
	pipe.Expire(ctx, key, 2*time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return &RateLimitResult{Allowed: true, Remaining: -1}, nil
	}

	count := int(countCmd.Val())
	if count >= cfg.RPM {
		rl.rdb.ZRem(ctx, key, member)
		return &RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			Limit:      cfg.RPM,
			RetryAfter: time.Second,
		}, nil
	}

	return &RateLimitResult{
		Allowed:   true,
		Remaining: cfg.RPM - count - 1,
		Limit:     cfg.RPM,
	}, nil
}

// AcquireConcurrency tries to acquire a concurrency slot.
func (rl *RateLimiter) AcquireConcurrency(ctx context.Context, tenantID string) (bool, error) {
	cfg := rl.getConfig(ctx, tenantID)
	if rl.rdb == nil || cfg.MaxConcurrent <= 0 {
		return true, nil
	}

	key := fmt.Sprintf("rl:conc:%s", tenantID)
	val, err := rl.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, nil
	}

	if val == 1 {
		rl.rdb.Expire(ctx, key, 10*time.Minute)
	}

	if int(val) > cfg.MaxConcurrent {
		rl.rdb.Decr(ctx, key)
		return false, nil
	}

	return true, nil
}

// ReleaseConcurrency releases a concurrency slot.
func (rl *RateLimiter) ReleaseConcurrency(ctx context.Context, tenantID string) {
	cfg := rl.getConfig(ctx, tenantID)
	if rl.rdb == nil || cfg.MaxConcurrent <= 0 {
		return
	}
	key := fmt.Sprintf("rl:conc:%s", tenantID)
	rl.rdb.Decr(ctx, key)
}

// Check performs both RPM and concurrency checks.
func (rl *RateLimiter) Check(ctx context.Context, tenantID string) (*RateLimitResult, error) {
	rpmResult, err := rl.CheckRPM(ctx, tenantID)
	if err != nil {
		return rpmResult, err
	}
	if !rpmResult.Allowed {
		return rpmResult, nil
	}

	acquired, err := rl.AcquireConcurrency(ctx, tenantID)
	if err != nil {
		return &RateLimitResult{Allowed: true, Remaining: -1}, nil
	}
	if !acquired {
		return &RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			Limit:      rl.getConfig(ctx, tenantID).MaxConcurrent,
			RetryAfter: time.Second,
		}, nil
	}

	return rpmResult, nil
}

// Release releases the concurrency slot for a tenant.
func (rl *RateLimiter) Release(ctx context.Context, tenantID string) {
	rl.ReleaseConcurrency(ctx, tenantID)
}

// GetConfig returns the rate limit config for a tenant from the DB.
// Returns nil if no custom config exists.
func (rl *RateLimiter) GetConfig(ctx context.Context, tenantID kernel.TenantID) (*RateLimitConfig, error) {
	if rl.configRepo == nil {
		return nil, nil
	}
	return rl.configRepo.GetByTenantID(ctx, tenantID)
}

// UpsertConfig creates or updates the rate limit config for a tenant.
func (rl *RateLimiter) UpsertConfig(ctx context.Context, cfg *RateLimitConfig) (*RateLimitConfig, error) {
	if rl.configRepo == nil {
		return nil, fmt.Errorf("no config repository configured")
	}
	result, err := rl.configRepo.Upsert(ctx, cfg)
	if err != nil {
		return nil, err
	}
	rl.InvalidateCache(cfg.TenantID.String())
	return result, nil
}

// DeleteConfig removes the custom rate limit config for a tenant.
func (rl *RateLimiter) DeleteConfig(ctx context.Context, tenantID kernel.TenantID) error {
	if rl.configRepo == nil {
		return fmt.Errorf("no config repository configured")
	}
	err := rl.configRepo.Delete(ctx, tenantID)
	if err != nil {
		return err
	}
	rl.InvalidateCache(tenantID.String())
	return nil
}
