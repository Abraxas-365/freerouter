package webhook

import (
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Webhook Config Entity
// ============================================================================

type WebhookConfig struct {
	ID        string          `db:"id" json:"id"`
	TenantID  kernel.TenantID `db:"tenant_id" json:"tenant_id"`
	URL       string          `db:"url" json:"url"`
	Secret    string          `db:"secret" json:"-"` // never exposed in API
	Events    []string        `db:"events" json:"events"`
	Enabled   bool            `db:"enabled" json:"enabled"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// CreateWebhookRequest is the DTO for creating a webhook.
type CreateWebhookRequest struct {
	URL    string   `json:"url" validate:"required,url"`
	Events []string `json:"events" validate:"required,min=1"`
}

func (r *CreateWebhookRequest) Validate() error {
	if r.URL == "" {
		return errx.Validation("url is required").WithDetail("field", "url")
	}
	if len(r.Events) == 0 {
		return errx.Validation("at least one event is required").WithDetail("field", "events")
	}
	for _, e := range r.Events {
		if !IsValidEvent(e) {
			return errx.Validation("invalid event type: " + e).WithDetail("field", "events")
		}
	}
	return nil
}

// UpdateWebhookRequest is the DTO for updating a webhook.
type UpdateWebhookRequest struct {
	URL     *string  `json:"url,omitempty"`
	Events  []string `json:"events,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// ============================================================================
// Webhook Delivery Entity
// ============================================================================

type DeliveryStatus string

const (
	DeliveryPending DeliveryStatus = "pending"
	DeliverySuccess DeliveryStatus = "success"
	DeliveryFailed  DeliveryStatus = "failed"
)

type WebhookDelivery struct {
	ID          string         `db:"id" json:"id"`
	WebhookID   string         `db:"webhook_id" json:"webhook_id"`
	EventType   string         `db:"event_type" json:"event_type"`
	Payload     string         `db:"payload" json:"payload"`
	Status      DeliveryStatus `db:"status" json:"status"`
	StatusCode  *int           `db:"status_code" json:"status_code,omitempty"`
	Attempts    int            `db:"attempts" json:"attempts"`
	LastError   *string        `db:"last_error" json:"last_error,omitempty"`
	NextRetryAt *time.Time     `db:"next_retry_at" json:"next_retry_at,omitempty"`
	CreatedAt   time.Time      `db:"created_at" json:"created_at"`
	CompletedAt *time.Time     `db:"completed_at" json:"completed_at,omitempty"`
}

// ============================================================================
// Event Types
// ============================================================================

const (
	EventRequestCompleted  = "request.completed"
	EventRequestFailed     = "request.failed"
	EventSpendingWarning   = "spending.warning"
	EventSpendingExceeded  = "spending.exceeded"
	EventKeyHealthDegraded = "key.health_degraded"
	EventKeyBlacklisted    = "key.blacklisted"
)

var validEvents = map[string]bool{
	EventRequestCompleted:  true,
	EventRequestFailed:     true,
	EventSpendingWarning:   true,
	EventSpendingExceeded:  true,
	EventKeyHealthDegraded: true,
	EventKeyBlacklisted:    true,
}

func IsValidEvent(event string) bool {
	return validEvents[event]
}

func AllEvents() []string {
	return []string{
		EventRequestCompleted,
		EventRequestFailed,
		EventSpendingWarning,
		EventSpendingExceeded,
		EventKeyHealthDegraded,
		EventKeyBlacklisted,
	}
}

// WebhookPayload is the envelope sent to webhook endpoints.
type WebhookPayload struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	TenantID  string    `json:"tenant_id"`
	Data      any       `json:"data"`
}

// ============================================================================
// Errors
// ============================================================================

var ErrRegistry = errx.NewRegistry("WEBHOOK")

var (
	CodeWebhookNotFound = ErrRegistry.Register("WEBHOOK_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Webhook not found")
)

func ErrWebhookNotFound() *errx.Error { return ErrRegistry.New(CodeWebhookNotFound) }
