package routingconfigsvc

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/routingconfig"
)

var (
	_ routingconfig.Commands = (*Service)(nil)
	_ routingconfig.Queries  = (*Service)(nil)
	_ routingconfig.Resolver = (*Service)(nil)
)

const configCacheTTL = 30 * time.Second

// Service implements routing config CRUD and exposes a Resolve method for
// the gateway to call on every request. Config lookups are cached in-memory
// with a short TTL so the hot path never hits Postgres.
type Service struct {
	repo routingconfig.Repository

	// in-memory config cache (subject → cached entry)
	cache sync.Map
}

type cachedConfig struct {
	cfg       routingconfig.RoutingConfig
	expiresAt time.Time
}

// New creates a routing config service.
func New(repo routingconfig.Repository) *Service {
	return &Service{repo: repo}
}

// ── Commands ────────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, cmd routingconfig.CreateRoutingConfig) (routingconfig.RoutingConfig, error) {
	if err := cmd.Validate(); err != nil {
		return routingconfig.RoutingConfig{}, err
	}

	now := time.Now().UTC()
	cfg := routingconfig.RoutingConfig{
		ID:        identity.NewRoutingConfigID(),
		SubjectID: cmd.SubjectID,
		Strategy:  cmd.Strategy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, cfg); err != nil {
		return routingconfig.RoutingConfig{}, err
	}

	s.invalidateCache(cfg.SubjectID)
	return cfg, nil
}

func (s *Service) Update(ctx context.Context, id identity.RoutingConfigID, cmd routingconfig.UpdateRoutingConfig) (routingconfig.RoutingConfig, error) {
	if err := cmd.Validate(); err != nil {
		return routingconfig.RoutingConfig{}, err
	}

	cfg, err := s.repo.Find(ctx, id)
	if err != nil {
		return routingconfig.RoutingConfig{}, err
	}

	if cmd.Strategy != nil {
		cfg.Strategy = *cmd.Strategy
	}
	cfg.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, cfg); err != nil {
		return routingconfig.RoutingConfig{}, err
	}

	s.invalidateCache(cfg.SubjectID)
	return cfg, nil
}

func (s *Service) Delete(ctx context.Context, id identity.RoutingConfigID) error {
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

func (s *Service) Find(ctx context.Context, id identity.RoutingConfigID) (routingconfig.RoutingConfig, error) {
	return s.repo.Find(ctx, id)
}

func (s *Service) FindBySubject(ctx context.Context, subjectID string) (routingconfig.RoutingConfig, error) {
	return s.repo.FindBySubject(ctx, subjectID)
}

func (s *Service) List(ctx context.Context, filter routingconfig.Filter, page query.Pagination) (query.Paginated[routingconfig.RoutingConfig], error) {
	return s.repo.List(ctx, filter, page)
}

// ── Gateway-facing resolution ───────────────────────────────────────

// Resolve returns the effective routing strategy for a subject. It never
// errors: on any lookup failure it falls back to the "default" subject
// config, and finally to routingconfig.DefaultStrategy.
func (s *Service) Resolve(ctx context.Context, subjectID string) routingconfig.Strategy {
	cfg := s.resolveConfig(ctx, subjectID)
	return cfg.Strategy
}

// ── config cache ────────────────────────────────────────────────────

// resolveConfig looks up subject-specific config, falls back to "default",
// and finally uses the hard-coded default strategy.
func (s *Service) resolveConfig(ctx context.Context, subjectID string) routingconfig.RoutingConfig {
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
	if cfg, ok := s.loadCached(routingconfig.DefaultSubjectID); ok {
		// Also cache under the subject key so we don't re-lookup
		s.storeCache(subjectID, cfg)
		return cfg
	}

	// DB lookup: default
	cfg, err = s.repo.FindBySubject(ctx, routingconfig.DefaultSubjectID)
	if err == nil {
		s.storeCache(routingconfig.DefaultSubjectID, cfg)
		s.storeCache(subjectID, cfg)
		return cfg
	}

	// Hard-coded fallback
	slog.Debug("no routing config found, using default strategy", "subject", subjectID)
	fallback := routingconfig.RoutingConfig{Strategy: routingconfig.DefaultStrategy}
	s.storeCache(subjectID, fallback)
	return fallback
}

func (s *Service) loadCached(key string) (routingconfig.RoutingConfig, bool) {
	raw, ok := s.cache.Load(key)
	if !ok {
		return routingconfig.RoutingConfig{}, false
	}
	cc := raw.(*cachedConfig)
	if time.Now().After(cc.expiresAt) {
		s.cache.Delete(key)
		return routingconfig.RoutingConfig{}, false
	}
	return cc.cfg, true
}

func (s *Service) storeCache(key string, cfg routingconfig.RoutingConfig) {
	s.cache.Store(key, &cachedConfig{cfg: cfg, expiresAt: time.Now().Add(configCacheTTL)})
}

func (s *Service) invalidateCache(subjectID string) {
	s.cache.Delete(subjectID)
	// Also clear any subject entries that might have inherited this default
	if subjectID == routingconfig.DefaultSubjectID {
		s.cache.Range(func(key, _ any) bool {
			s.cache.Delete(key)
			return true
		})
	}
}
