package ratelimitsvc

import (
	"context"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/ratelimit"
)

var (
	_ ratelimit.Commands = (*Service)(nil)
	_ ratelimit.Queries  = (*Service)(nil)
)

// Service implements rate limit config CRUD and exposes a Check/Release
// pair for the gateway to call. Config lookups are cached in-memory with
// a short TTL so the hot path never hits Postgres.
type Service struct {
	repo    ratelimit.Repository
	limiter ratelimit.Limiter

	// in-memory config cache (subject → cached entry)
	cache sync.Map
}

type cachedConfig struct {
	cfg       ratelimit.RateLimitConfig
	expiresAt time.Time
}

const configCacheTTL = 30 * time.Second

// New creates a rate limit service.
func New(repo ratelimit.Repository, limiter ratelimit.Limiter) *Service {
	return &Service{repo: repo, limiter: limiter}
}

// ── Commands ────────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, cmd ratelimit.CreateRateLimitConfig) (ratelimit.RateLimitConfig, error) {
	if err := cmd.Validate(); err != nil {
		return ratelimit.RateLimitConfig{}, err
	}

	now := time.Now().UTC()
	cfg := ratelimit.RateLimitConfig{
		ID:            identity.NewRateLimitConfigID(),
		Name:          cmd.Name,
		SubjectID:     cmd.SubjectID,
		RPM:           cmd.RPM,
		MaxConcurrent: cmd.MaxConcurrent,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.Create(ctx, cfg); err != nil {
		return ratelimit.RateLimitConfig{}, err
	}

	s.invalidateCache(cfg.SubjectID)
	return cfg, nil
}

func (s *Service) Update(ctx context.Context, id identity.RateLimitConfigID, cmd ratelimit.UpdateRateLimitConfig) (ratelimit.RateLimitConfig, error) {
	if err := cmd.Validate(); err != nil {
		return ratelimit.RateLimitConfig{}, err
	}

	cfg, err := s.repo.Find(ctx, id)
	if err != nil {
		return ratelimit.RateLimitConfig{}, err
	}

	if cmd.Name != nil {
		cfg.Name = *cmd.Name
	}
	if cmd.RPM != nil {
		cfg.RPM = *cmd.RPM
	}
	if cmd.MaxConcurrent != nil {
		cfg.MaxConcurrent = *cmd.MaxConcurrent
	}
	cfg.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, cfg); err != nil {
		return ratelimit.RateLimitConfig{}, err
	}

	s.invalidateCache(cfg.SubjectID)
	return cfg, nil
}

func (s *Service) Delete(ctx context.Context, id identity.RateLimitConfigID) error {
	cfg, err := s.repo.Find(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateCache(cfg.SubjectID)
	return nil
}

// ── Queries ─────────────────────────────────────────────────────────

func (s *Service) Find(ctx context.Context, id identity.RateLimitConfigID) (ratelimit.RateLimitConfig, error) {
	return s.repo.Find(ctx, id)
}

func (s *Service) FindBySubject(ctx context.Context, subjectID string) (ratelimit.RateLimitConfig, error) {
	return s.repo.FindBySubject(ctx, subjectID)
}

func (s *Service) List(ctx context.Context, filter ratelimit.Filter, page query.Pagination) (query.Paginated[ratelimit.RateLimitConfig], error) {
	return s.repo.List(ctx, filter, page)
}

// ── Gateway-facing check / release ─────────────────────────────────

// Check resolves the effective config for the subject and delegates to
// the limiter. Returns the result. Caller MUST call Release on success.
func (s *Service) Check(ctx context.Context, subjectID string) (*ratelimit.RateLimitResult, error) {
	cfg := s.resolveConfig(ctx, subjectID)
	if cfg.RPM == 0 && cfg.MaxConcurrent == 0 {
		// Both unlimited
		return &ratelimit.RateLimitResult{Allowed: true, Limit: 0}, nil
	}
	if s.limiter == nil {
		return nil, errx.New("rate limit backend unavailable", errx.TypeExternal)
	}
	return s.limiter.Check(ctx, subjectID, cfg.RPM, cfg.MaxConcurrent)
}

// Release frees the concurrency slot for the subject.
func (s *Service) Release(ctx context.Context, subjectID string) {
	if s.limiter == nil {
		return
	}
	s.limiter.Release(ctx, subjectID)
}

// ── config cache ────────────────────────────────────────────────────

// resolveConfig looks up subject-specific config, falls back to "default",
// and finally uses hard-coded defaults.
func (s *Service) resolveConfig(ctx context.Context, subjectID string) ratelimit.RateLimitConfig {
	// Try subject-specific config (cached)
	if cfg, ok := s.loadCached(subjectID); ok {
		return cfg
	}

	// DB lookup: subject-specific
	cfg, err := s.repo.FindBySubject(ctx, subjectID)
	if err == nil {
		s.storeCache(subjectID, cfg)
		return cfg
	}

	// Try "default" config (cached)
	if cfg, ok := s.loadCached(ratelimit.DefaultSubjectID); ok {
		// Also cache under the subject key so we don't re-lookup
		s.storeCache(subjectID, cfg)
		return cfg
	}

	// DB lookup: default
	cfg, err = s.repo.FindBySubject(ctx, ratelimit.DefaultSubjectID)
	if err == nil {
		s.storeCache(ratelimit.DefaultSubjectID, cfg)
		s.storeCache(subjectID, cfg)
		return cfg
	}

	// Hard-coded fallback
	fallback := ratelimit.RateLimitConfig{
		RPM:           ratelimit.DefaultRPM,
		MaxConcurrent: ratelimit.DefaultMaxConcurrent,
	}
	s.storeCache(subjectID, fallback)
	return fallback
}

func (s *Service) loadCached(key string) (ratelimit.RateLimitConfig, bool) {
	raw, ok := s.cache.Load(key)
	if !ok {
		return ratelimit.RateLimitConfig{}, false
	}
	cc := raw.(*cachedConfig)
	if time.Now().After(cc.expiresAt) {
		s.cache.Delete(key)
		return ratelimit.RateLimitConfig{}, false
	}
	return cc.cfg, true
}

func (s *Service) storeCache(key string, cfg ratelimit.RateLimitConfig) {
	s.cache.Store(key, &cachedConfig{cfg: cfg, expiresAt: time.Now().Add(configCacheTTL)})
}

func (s *Service) invalidateCache(subjectID string) {
	s.cache.Delete(subjectID)
	// Also clear any subject entries that might have inherited this default
	if subjectID == ratelimit.DefaultSubjectID {
		s.cache.Range(func(key, _ any) bool {
			s.cache.Delete(key)
			return true
		})
	}
}
