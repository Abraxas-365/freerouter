package usagesvc

import (
	"context"
	"log/slog"
	"time"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/usage"
)

// Service implements usage.Commands and usage.Queries.
// LogRequest is non-blocking — logs are persisted by a background goroutine.
type Service struct {
	repo      usage.Repository
	logsCh    chan usage.UsageLog
	purgeStop chan struct{}
}

var _ usage.Commands = (*Service)(nil)
var _ usage.Queries = (*Service)(nil)

// New creates a usage service with an async log buffer.
func New(repo usage.Repository, bufferSize int) *Service {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	s := &Service{
		repo:   repo,
		logsCh: make(chan usage.UsageLog, bufferSize),
	}
	go s.processLogs()
	return s
}

// LogRequest enqueues a usage log for async persistence.
func (s *Service) LogRequest(log usage.UsageLog) {
	select {
	case s.logsCh <- log:
	default:
		slog.Warn("usage log buffer full, dropping log", "log_id", log.ID)
	}
}

// Close drains remaining logs and shuts down the background worker.
func (s *Service) Close() {
	close(s.logsCh)
}

func (s *Service) processLogs() {
	for log := range s.logsCh {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.repo.Create(ctx, log); err != nil {
			slog.Error("failed to persist usage log", "log_id", log.ID, "error", err)
		}
		cancel()
	}
}

// ── Queries ──────────────────────────────────────────────────────────

func (s *Service) Find(ctx context.Context, id identity.UsageLogID) (usage.UsageLog, error) {
	return s.repo.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, filter usage.Filter, page query.Pagination) (query.Paginated[usage.UsageLog], error) {
	return s.repo.List(ctx, filter, page.Normalize())
}

func (s *Service) GetSummary(ctx context.Context, from, to *time.Time) (*usage.SummaryResponse, error) {
	summary, err := s.repo.GetSummary(ctx, from, to)
	if err != nil {
		return nil, err
	}

	byModel, err := s.repo.GetSummaryByModel(ctx, from, to)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	periodStart := now.AddDate(0, -1, 0)
	periodEnd := now
	if from != nil {
		periodStart = *from
	}
	if to != nil {
		periodEnd = *to
	}

	return &usage.SummaryResponse{
		Summary:     *summary,
		ByModel:     byModel,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}, nil
}

// ── Retention config ───────────────────────────────────────────────

func (s *Service) GetRetention(ctx context.Context) (usage.RetentionConfig, error) {
	return s.repo.GetRetention(ctx)
}

func (s *Service) UpsertRetention(ctx context.Context, cmd usage.UpsertRetention) (usage.RetentionConfig, error) {
	if err := cmd.Validate(); err != nil {
		return usage.RetentionConfig{}, err
	}

	existing, err := s.repo.GetRetention(ctx)
	now := time.Now().UTC()

	if err != nil {
		// No config yet — create with defaults + overrides.
		defaults := usage.DefaultRetentionConfig()
		if cmd.RetentionDays != nil {
			defaults.RetentionDays = *cmd.RetentionDays
		}
		if cmd.RetainMessages != nil {
			defaults.RetainMessages = *cmd.RetainMessages
		}
		if cmd.RetainResponseBody != nil {
			defaults.RetainResponseBody = *cmd.RetainResponseBody
		}

		cfg := usage.RetentionConfig{
			ID:                 identity.NewRetentionConfigID(),
			RetentionDays:      defaults.RetentionDays,
			RetainMessages:     defaults.RetainMessages,
			RetainResponseBody: defaults.RetainResponseBody,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		if err := s.repo.UpsertRetention(ctx, cfg); err != nil {
			return usage.RetentionConfig{}, err
		}
		return cfg, nil
	}

	if cmd.RetentionDays != nil {
		existing.RetentionDays = *cmd.RetentionDays
	}
	if cmd.RetainMessages != nil {
		existing.RetainMessages = *cmd.RetainMessages
	}
	if cmd.RetainResponseBody != nil {
		existing.RetainResponseBody = *cmd.RetainResponseBody
	}
	existing.UpdatedAt = now

	if err := s.repo.UpsertRetention(ctx, existing); err != nil {
		return usage.RetentionConfig{}, err
	}
	return existing, nil
}

func (s *Service) DeleteRetention(ctx context.Context) error {
	return s.repo.DeleteRetention(ctx)
}

// ── Purge worker ───────────────────────────────────────────────────

// StartPurgeWorker launches a background goroutine that periodically purges
// usage logs older than the configured retention period. No-op if no
// retention config exists or retention_days is 0 (retain forever).
func (s *Service) StartPurgeWorker(interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	s.purgeStop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runPurge()
			case <-s.purgeStop:
				return
			}
		}
	}()
}

// StopPurgeWorker stops the background purge worker started by StartPurgeWorker.
func (s *Service) StopPurgeWorker() {
	if s.purgeStop != nil {
		close(s.purgeStop)
		s.purgeStop = nil
	}
}

func (s *Service) runPurge() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg, err := s.repo.GetRetention(ctx)
	if err != nil {
		// No config = retain everything.
		return
	}
	if cfg.RetentionDays <= 0 {
		return
	}

	n, err := s.repo.PurgeOlderThan(ctx, cfg.RetentionDays)
	if err != nil {
		slog.Error("usage log purge failed", "error", err)
		return
	}
	if n > 0 {
		slog.Info("purged usage logs", "count", n, "older_than_days", cfg.RetentionDays)
	}
}
