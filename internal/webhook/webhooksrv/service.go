package webhooksrv

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/webhook"
	"github.com/google/uuid"
)

const (
	maxRetries     = 5
	deliveryTimeout = 10 * time.Second
	retryInterval  = 30 * time.Second
	pollInterval   = 10 * time.Second
	maxBatchSize   = 50
)

type WebhookService struct {
	repo   webhook.WebhookRepository
	client *http.Client
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewWebhookService(repo webhook.WebhookRepository) *WebhookService {
	return &WebhookService{
		repo: repo,
		client: &http.Client{Timeout: deliveryTimeout},
		stopCh: make(chan struct{}),
	}
}

// StartWorker starts the background retry worker.
func (s *WebhookService) StartWorker() {
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
				s.processRetries()
			}
		}
	}()
}

// Stop gracefully stops the background worker.
func (s *WebhookService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// ============================================================================
// CRUD
// ============================================================================

func (s *WebhookService) Create(ctx context.Context, tenantID kernel.TenantID, req webhook.CreateWebhookRequest) (*webhook.WebhookConfig, error) {
	secret := generateSecret()
	w := &webhook.WebhookConfig{
		ID:        kernel.NewWebhookID(uuid.NewString()),
		TenantID:  tenantID,
		URL:       req.URL,
		Secret:    secret,
		Events:    req.Events,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *WebhookService) Get(ctx context.Context, id kernel.WebhookID) (*webhook.WebhookConfig, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *WebhookService) List(ctx context.Context, tenantID kernel.TenantID) ([]*webhook.WebhookConfig, error) {
	return s.repo.FindByTenant(ctx, tenantID)
}

func (s *WebhookService) Update(ctx context.Context, id kernel.WebhookID, req webhook.UpdateWebhookRequest) (*webhook.WebhookConfig, error) {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.URL != nil {
		w.URL = *req.URL
	}
	if req.Events != nil {
		w.Events = req.Events
	}
	if req.Enabled != nil {
		w.Enabled = *req.Enabled
	}
	w.UpdatedAt = time.Now().UTC()
	if err := s.repo.Save(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *WebhookService) Delete(ctx context.Context, id kernel.WebhookID) error {
	return s.repo.Delete(ctx, id)
}

func (s *WebhookService) GetDeliveries(ctx context.Context, webhookID kernel.WebhookID, limit int) ([]*webhook.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.FindDeliveriesByWebhook(ctx, webhookID, limit)
}

// ============================================================================
// Event Dispatch
// ============================================================================

// Fire enqueues a webhook event for all matching tenant webhooks.
// Delivery happens asynchronously — this method returns immediately.
func (s *WebhookService) Fire(tenantID kernel.TenantID, event string, data any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		configs, err := s.repo.FindEnabledByTenantAndEvent(ctx, tenantID, event)
		if err != nil {
			slog.Error("webhook: failed to find configs", "tenant_id", tenantID, "event", event, "error", err)
			return
		}

		payload := webhook.WebhookPayload{
			ID:        uuid.NewString(),
			Event:     event,
			Timestamp: time.Now().UTC(),
			TenantID:  tenantID.String(),
			Data:      data,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			slog.Error("webhook: failed to marshal payload", "error", err)
			return
		}

		for _, cfg := range configs {
			delivery := &webhook.WebhookDelivery{
				ID:        kernel.NewWebhookDeliveryID(uuid.NewString()),
				WebhookID: cfg.ID,
				EventType: event,
				Payload:   string(payloadJSON),
				Status:    webhook.DeliveryPending,
				Attempts:  0,
				CreatedAt: time.Now().UTC(),
			}

			if err := s.repo.SaveDelivery(ctx, delivery); err != nil {
				slog.Error("webhook: failed to save delivery", "error", err)
				continue
			}

			// Attempt immediate delivery
			go s.deliver(cfg, delivery)
		}
	}()
}

// ============================================================================
// Delivery
// ============================================================================

func (s *WebhookService) deliver(cfg *webhook.WebhookConfig, d *webhook.WebhookDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout+2*time.Second)
	defer cancel()

	d.Attempts++

	// Sign the payload
	sig := sign(d.Payload, cfg.Secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.URL, bytes.NewBufferString(d.Payload))
	if err != nil {
		s.markFailed(ctx, d, 0, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-ID", d.WebhookID.String())
	req.Header.Set("X-Webhook-Signature", sig)
	req.Header.Set("X-Webhook-Event", d.EventType)

	resp, err := s.client.Do(req)
	if err != nil {
		s.scheduleRetry(ctx, d, 0, err)
		return
	}
	defer resp.Body.Close()
	io.ReadAll(io.LimitReader(resp.Body, 1024)) // drain

	statusCode := resp.StatusCode
	d.StatusCode = &statusCode

	if statusCode >= 200 && statusCode < 300 {
		now := time.Now().UTC()
		d.Status = webhook.DeliverySuccess
		d.CompletedAt = &now
		s.repo.UpdateDelivery(ctx, d)
		return
	}

	s.scheduleRetry(ctx, d, statusCode, fmt.Errorf("HTTP %d", statusCode))
}

func (s *WebhookService) scheduleRetry(ctx context.Context, d *webhook.WebhookDelivery, statusCode int, err error) {
	if d.Attempts >= maxRetries {
		s.markFailed(ctx, d, statusCode, err)
		return
	}
	errStr := err.Error()
	d.LastError = &errStr
	// Exponential backoff: 30s, 60s, 120s, 240s, 480s
	delay := retryInterval * time.Duration(1<<(d.Attempts-1))
	nextRetry := time.Now().UTC().Add(delay)
	d.NextRetryAt = &nextRetry
	if statusCode > 0 {
		d.StatusCode = &statusCode
	}
	s.repo.UpdateDelivery(ctx, d)
}

func (s *WebhookService) markFailed(ctx context.Context, d *webhook.WebhookDelivery, statusCode int, err error) {
	now := time.Now().UTC()
	errStr := err.Error()
	d.Status = webhook.DeliveryFailed
	d.LastError = &errStr
	d.CompletedAt = &now
	if statusCode > 0 {
		d.StatusCode = &statusCode
	}
	s.repo.UpdateDelivery(ctx, d)
}

func (s *WebhookService) processRetries() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deliveries, err := s.repo.FindPendingDeliveries(ctx, maxBatchSize)
	if err != nil {
		slog.Error("webhook: failed to find pending deliveries", "error", err)
		return
	}

	for _, d := range deliveries {
		cfg, err := s.repo.FindByID(ctx, d.WebhookID)
		if err != nil {
			s.markFailed(ctx, d, 0, fmt.Errorf("webhook config not found"))
			continue
		}
		s.deliver(cfg, d)
	}
}

// ============================================================================
// Helpers
// ============================================================================

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func generateSecret() string {
	id := uuid.NewString()
	mac := hmac.New(sha256.New, []byte(id))
	mac.Write([]byte(time.Now().String()))
	return "whsec_" + hex.EncodeToString(mac.Sum(nil))[:32]
}

