package webhooksvc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
	"github.com/Abraxas-365/freerouter/internal/webhook"
)

const (
	maxRetries      = 5
	deliveryTimeout = 10 * time.Second
	retryInterval   = 30 * time.Second
	pollInterval    = 10 * time.Second
	maxBatchSize    = 50
)

var (
	_ webhook.Commands   = (*Service)(nil)
	_ webhook.Queries    = (*Service)(nil)
	_ webhook.Dispatcher = (*Service)(nil)
)

// Service implements webhook config/delivery CRUD plus signed delivery with
// retry. Fire is fire-and-forget: it enqueues a delivery record and returns;
// actual HTTP delivery + retries happen on a background worker goroutine.
type Service struct {
	repo   webhook.Repository
	client *http.Client
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New creates a webhook service. Call StartWorker to begin processing
// pending/retryable deliveries, and Stop to shut it down gracefully.
func New(repo webhook.Repository) *Service {
	return &Service{
		repo:   repo,
		client: publicWebhookClient(deliveryTimeout),
		stopCh: make(chan struct{}),
	}
}

// StartWorker starts the background delivery retry loop.
func (s *Service) StartWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.processPending()
			}
		}
	}()
}

// Stop gracefully stops the background worker.
func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// ── Commands ────────────────────────────────────────────────────────

func (s *Service) Create(ctx context.Context, cmd webhook.CreateWebhook) (webhook.WebhookConfig, error) {
	if err := cmd.Validate(); err != nil {
		return webhook.WebhookConfig{}, err
	}

	now := time.Now().UTC()
	cfg := webhook.WebhookConfig{
		ID:        identity.NewWebhookID(),
		URL:       cmd.URL,
		Secret:    generateSecret(),
		Events:    cmd.Events,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return webhook.WebhookConfig{}, err
	}
	return cfg, nil
}

func (s *Service) Update(ctx context.Context, id identity.WebhookID, cmd webhook.UpdateWebhook) (webhook.WebhookConfig, error) {
	if err := cmd.Validate(); err != nil {
		return webhook.WebhookConfig{}, err
	}

	cfg, err := s.repo.Find(ctx, id)
	if err != nil {
		return webhook.WebhookConfig{}, err
	}

	if cmd.URL != nil {
		cfg.URL = *cmd.URL
	}
	if cmd.Events != nil {
		cfg.Events = cmd.Events
	}
	if cmd.Enabled != nil {
		cfg.Enabled = *cmd.Enabled
	}
	cfg.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, cfg); err != nil {
		return webhook.WebhookConfig{}, err
	}
	return cfg, nil
}

func (s *Service) Delete(ctx context.Context, id identity.WebhookID) error {
	return s.repo.Delete(ctx, id)
}

// ── Queries ─────────────────────────────────────────────────────────

func (s *Service) Find(ctx context.Context, id identity.WebhookID) (webhook.WebhookConfig, error) {
	return s.repo.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, page query.Pagination) (query.Paginated[webhook.WebhookConfig], error) {
	return s.repo.List(ctx, page.Normalize())
}

func (s *Service) ListDeliveries(ctx context.Context, webhookID identity.WebhookID, page query.Pagination) (query.Paginated[webhook.WebhookDelivery], error) {
	return s.repo.ListDeliveries(ctx, webhookID, page.Normalize())
}

// ── Dispatch ────────────────────────────────────────────────────────

// Fire enqueues delivery of event to every enabled webhook subscribed to it.
// Best-effort: lookup/persist failures are logged, never surfaced to the caller.
func (s *Service) Fire(event string, data any) {
	go s.fire(event, data)
}

func (s *Service) fire(event string, data any) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configs, err := s.repo.FindEnabledByEvent(ctx, event)
	if err != nil {
		slog.Error("webhook: failed to find subscribers", "event", event, "error", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	payload := webhook.WebhookPayload{
		ID:        identity.NewWebhookDeliveryID().String(),
		Event:     event,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhook: failed to marshal payload", "event", event, "error", err)
		return
	}

	for _, cfg := range configs {
		delivery := webhook.WebhookDelivery{
			ID:        identity.NewWebhookDeliveryID(),
			WebhookID: cfg.ID,
			EventType: event,
			Payload:   string(body),
			Status:    webhook.DeliveryPending,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.repo.SaveDelivery(ctx, delivery); err != nil {
			slog.Error("webhook: failed to save delivery", "webhook_id", cfg.ID, "error", err)
			continue
		}
		s.deliver(cfg, delivery)
	}
}

// deliver attempts one HTTP delivery and updates the delivery record.
func (s *Service) deliver(cfg webhook.WebhookConfig, d webhook.WebhookDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout)
	defer cancel()

	d.Attempts++

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewReader([]byte(d.Payload)))
	if err != nil {
		s.markFailed(ctx, d, 0, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", d.EventType)
	req.Header.Set("X-Webhook-Signature", sign(d.Payload, cfg.Secret))

	resp, err := s.client.Do(req)
	if err != nil {
		s.markFailed(ctx, d, 0, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	status := resp.StatusCode
	if status >= 200 && status < 300 {
		now := time.Now().UTC()
		d.Status = webhook.DeliverySuccess
		d.StatusCode = &status
		d.CompletedAt = &now
		if err := s.repo.UpdateDelivery(ctx, d); err != nil {
			slog.Error("webhook: failed to update delivery", "delivery_id", d.ID, "error", err)
		}
		return
	}

	s.markFailed(ctx, d, status, fmt.Errorf("upstream returned status %d", status))
}

func (s *Service) markFailed(ctx context.Context, d webhook.WebhookDelivery, status int, cause error) {
	errMsg := cause.Error()
	d.LastError = &errMsg
	if status != 0 {
		d.StatusCode = &status
	}

	if d.Attempts >= maxRetries {
		now := time.Now().UTC()
		d.Status = webhook.DeliveryFailed
		d.CompletedAt = &now
	} else {
		next := time.Now().UTC().Add(retryInterval * time.Duration(d.Attempts))
		d.Status = webhook.DeliveryPending
		d.NextRetryAt = &next
	}

	if err := s.repo.UpdateDelivery(ctx, d); err != nil {
		slog.Error("webhook: failed to update delivery", "delivery_id", d.ID, "error", err)
	}
}

// processPending re-attempts deliveries whose retry time has arrived.
func (s *Service) processPending() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deliveries, err := s.repo.FindPendingDeliveries(ctx, maxBatchSize)
	if err != nil {
		slog.Error("webhook: failed to find pending deliveries", "error", err)
		return
	}

	for _, d := range deliveries {
		cfg, err := s.repo.Find(ctx, d.WebhookID)
		if err != nil {
			s.markFailed(ctx, d, 0, fmt.Errorf("webhook config not found"))
			continue
		}
		s.deliver(cfg, d)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func generateSecret() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return "whsec_" + hex.EncodeToString(buf)
}
