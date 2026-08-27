package usagesrv

import (
	"context"
	"log/slog"
	"time"

	"github.com/Abraxas-365/freerouter/internal/ai/gateway"
	"github.com/Abraxas-365/freerouter/internal/ai/usage"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/google/uuid"
)

type UsageService struct {
	repo   usage.UsageRepository
	logsCh chan usage.UsageLog
}

func NewUsageService(repo usage.UsageRepository, bufferSize int) *UsageService {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	s := &UsageService{
		repo:   repo,
		logsCh: make(chan usage.UsageLog, bufferSize),
	}
	go s.processLogs()
	return s
}

// LogRequest builds and enqueues a usage log from a completed gateway request.
// It is non-blocking; logs are persisted asynchronously.
func (s *UsageService) LogRequest(
	tenantID kernel.TenantID,
	route *gateway.RouteResult,
	requestedModel string,
	resp *gateway.ChatResponse,
	statusCode int,
	duration time.Duration,
	streamed bool,
	reqErr error,
) {
	log := usage.UsageLog{
		ID:             kernel.NewUsageLogID(uuid.NewString()),
		TenantID:       tenantID,
		RequestedModel: requestedModel,
		UsedModel:      route.ExternalID,
		UsedProvider:   route.ProviderID,
		MappingID:      route.MappingID,
		DurationMs:     int(duration.Milliseconds()),
		Streamed:       streamed,
		StatusCode:     statusCode,
		CreatedAt:      time.Now().UTC(),
	}

	keyID := route.KeyID
	log.KeyID = &keyID

	if resp != nil && resp.Usage != nil {
		log.PromptTokens = resp.Usage.PromptTokens
		log.CompletionTokens = resp.Usage.CompletionTokens
		log.TotalTokens = resp.Usage.TotalTokens
		log.CachedTokens = resp.Usage.CacheReadInputTokens

		// Calculate costs (prices are per million tokens)
		if route.InputPrice != nil {
			log.InputCost = float64(log.PromptTokens) * *route.InputPrice / 1_000_000
		}
		if route.OutputPrice != nil {
			log.OutputCost = float64(log.CompletionTokens) * *route.OutputPrice / 1_000_000
		}
		log.TotalCost = log.InputCost + log.OutputCost

		if len(resp.Choices) > 0 && resp.Choices[0].FinishReason != nil {
			log.FinishReason = *resp.Choices[0].FinishReason
		}
	}

	if reqErr != nil {
		log.HasError = true
		log.ErrorMessage = reqErr.Error()
	}

	select {
	case s.logsCh <- log:
	default:
		slog.Warn("usage log buffer full, dropping log", "log_id", log.ID)
	}
}

// processLogs drains the buffer and persists logs.
func (s *UsageService) processLogs() {
	for log := range s.logsCh {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.repo.Create(ctx, log); err != nil {
			slog.Error("failed to persist usage log", "log_id", log.ID, "error", err)
		}
		cancel()
	}
}

// Close drains remaining logs and shuts down the background worker.
func (s *UsageService) Close() {
	close(s.logsCh)
}

// ============================================================================
// Query methods
// ============================================================================

func (s *UsageService) GetLog(ctx context.Context, id kernel.UsageLogID) (*usage.UsageLog, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UsageService) QueryLogs(ctx context.Context, q usage.UsageQuery) (*usage.UsageLogListResponse, error) {
	logs, total, err := s.repo.Query(ctx, q)
	if err != nil {
		return nil, errx.Wrap(err, "failed to query usage logs", errx.TypeInternal)
	}

	dtos := make([]usage.UsageLogDTO, len(logs))
	for i, l := range logs {
		dtos[i] = l.ToDTO()
	}
	return &usage.UsageLogListResponse{Logs: dtos, Total: total}, nil
}

func (s *UsageService) GetUsageSummary(ctx context.Context, tenantID kernel.TenantID, from, to *time.Time) (*usage.UsageSummaryResponse, error) {
	summary, err := s.repo.GetSummary(ctx, tenantID, from, to)
	if err != nil {
		return nil, errx.Wrap(err, "failed to get usage summary", errx.TypeInternal)
	}

	byModel, err := s.repo.GetSummaryByModel(ctx, tenantID, from, to)
	if err != nil {
		return nil, errx.Wrap(err, "failed to get usage by model", errx.TypeInternal)
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

	return &usage.UsageSummaryResponse{
		Summary:     *summary,
		ByModel:     byModel,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}, nil
}
