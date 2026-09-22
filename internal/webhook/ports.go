package webhook

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/identity"
	"github.com/Abraxas-365/freerouter/internal/query"
)

// ── Commands ────────────────────────────────────────────────────────

// Commands defines write operations for webhook subscriptions.
type Commands interface {
	Create(ctx context.Context, cmd CreateWebhook) (WebhookConfig, error)
	Update(ctx context.Context, id identity.WebhookID, cmd UpdateWebhook) (WebhookConfig, error)
	Delete(ctx context.Context, id identity.WebhookID) error
}

// ── Queries ─────────────────────────────────────────────────────────

// Queries defines read operations for webhook subscriptions and deliveries.
type Queries interface {
	Find(ctx context.Context, id identity.WebhookID) (WebhookConfig, error)
	List(ctx context.Context, page query.Pagination) (query.Paginated[WebhookConfig], error)
	ListDeliveries(ctx context.Context, webhookID identity.WebhookID, page query.Pagination) (query.Paginated[WebhookDelivery], error)
}

// Dispatcher is the gateway-facing hook used to fire webhook events.
// Fire is fire-and-forget: it enqueues delivery and returns immediately.
type Dispatcher interface {
	Fire(event string, data any)
}

// ── Repository ──────────────────────────────────────────────────────

// Repository is the persistence contract for webhook configs and deliveries.
type Repository interface {
	Find(ctx context.Context, id identity.WebhookID) (WebhookConfig, error)
	FindEnabledByEvent(ctx context.Context, event string) ([]WebhookConfig, error)
	List(ctx context.Context, page query.Pagination) (query.Paginated[WebhookConfig], error)
	Create(ctx context.Context, cfg WebhookConfig) error
	Update(ctx context.Context, cfg WebhookConfig) error
	Delete(ctx context.Context, id identity.WebhookID) error

	SaveDelivery(ctx context.Context, d WebhookDelivery) error
	UpdateDelivery(ctx context.Context, d WebhookDelivery) error
	FindPendingDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error)
	ListDeliveries(ctx context.Context, webhookID identity.WebhookID, page query.Pagination) (query.Paginated[WebhookDelivery], error)
}
