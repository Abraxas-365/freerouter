package webhook

import (
	"net/url"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/identity"
)

// ── Events ──────────────────────────────────────────────────────────

const (
	EventRequestCompleted  = "request.completed"
	EventRequestFailed     = "request.failed"
	EventKeyHealthDegraded = "key.health_degraded"
	EventKeyBlacklisted    = "key.blacklisted"
)

var validEvents = map[string]bool{
	EventRequestCompleted:  true,
	EventRequestFailed:     true,
	EventKeyHealthDegraded: true,
	EventKeyBlacklisted:    true,
}

// IsValidEvent reports whether event is a recognized webhook event type.
func IsValidEvent(event string) bool { return validEvents[event] }

// AllEvents returns every recognized webhook event type.
func AllEvents() []string {
	return []string{
		EventRequestCompleted,
		EventRequestFailed,
		EventKeyHealthDegraded,
		EventKeyBlacklisted,
	}
}

// ── WebhookConfig ───────────────────────────────────────────────────

// WebhookConfig is a subscription: a URL that receives signed POSTs for
// the given event types. Secret is used to HMAC-sign delivery payloads
// and is never exposed in API responses after creation.
type WebhookConfig struct {
	ID        identity.WebhookID `json:"id"                db:"id"`
	URL       string             `json:"url"                db:"url"`
	Secret    string             `json:"-"                  db:"secret"`
	Events    []string           `json:"events"             db:"events"`
	Enabled   bool               `json:"enabled"            db:"enabled"`
	CreatedAt time.Time          `json:"created_at"         db:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"         db:"updated_at"`
}

// ValidateURL rejects anything that isn't a plain HTTP(S) URL without
// embedded credentials or a fragment. This is a cheap first filter —
// SSRF protection against private/internal targets happens at dial time.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errx.Validation("webhook url must be http(s), without credentials or a fragment")
	}
	return nil
}

// ── CreateWebhook ───────────────────────────────────────────────────

// CreateWebhook is the input for creating a webhook subscription.
type CreateWebhook struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// Validate checks the URL and event list.
func (c CreateWebhook) Validate() error {
	if err := ValidateURL(c.URL); err != nil {
		return err
	}
	if len(c.Events) == 0 {
		return errx.Validation("at least one event is required")
	}
	for _, e := range c.Events {
		if !IsValidEvent(e) {
			return errx.Validation("invalid event type: " + e)
		}
	}
	return nil
}

// ── UpdateWebhook ───────────────────────────────────────────────────

// UpdateWebhook holds optional fields for updating a webhook subscription.
type UpdateWebhook struct {
	URL     *string  `json:"url,omitempty"`
	Events  []string `json:"events,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// Validate checks update fields if provided.
func (u UpdateWebhook) Validate() error {
	if u.URL != nil {
		if err := ValidateURL(*u.URL); err != nil {
			return err
		}
	}
	for _, e := range u.Events {
		if !IsValidEvent(e) {
			return errx.Validation("invalid event type: " + e)
		}
	}
	return nil
}

// ── WebhookDelivery ─────────────────────────────────────────────────

// DeliveryStatus is the outcome of a single delivery attempt cycle.
type DeliveryStatus string

const (
	DeliveryPending DeliveryStatus = "pending"
	DeliverySuccess DeliveryStatus = "success"
	DeliveryFailed  DeliveryStatus = "failed"
)

// WebhookDelivery records one event delivery to one webhook, including retries.
type WebhookDelivery struct {
	ID          identity.WebhookDeliveryID `json:"id"                     db:"id"`
	WebhookID   identity.WebhookID         `json:"webhook_id"             db:"webhook_id"`
	EventType   string                     `json:"event_type"             db:"event_type"`
	Payload     string                     `json:"payload"                db:"payload"`
	Status      DeliveryStatus             `json:"status"                 db:"status"`
	StatusCode  *int                       `json:"status_code,omitempty"  db:"status_code"`
	Attempts    int                        `json:"attempts"               db:"attempts"`
	LastError   *string                    `json:"last_error,omitempty"   db:"last_error"`
	NextRetryAt *time.Time                 `json:"next_retry_at,omitempty" db:"next_retry_at"`
	CreatedAt   time.Time                  `json:"created_at"             db:"created_at"`
	CompletedAt *time.Time                 `json:"completed_at,omitempty" db:"completed_at"`
}

// ── WebhookPayload ──────────────────────────────────────────────────

// WebhookPayload is the JSON envelope POSTed to subscriber endpoints.
type WebhookPayload struct {
	ID        string    `json:"id"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}
