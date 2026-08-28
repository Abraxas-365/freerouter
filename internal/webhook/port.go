package webhook

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// WebhookRepository defines the contract for webhook persistence.
type WebhookRepository interface {
	FindByID(ctx context.Context, id string) (*WebhookConfig, error)
	FindByTenant(ctx context.Context, tenantID kernel.TenantID) ([]*WebhookConfig, error)
	FindEnabledByTenantAndEvent(ctx context.Context, tenantID kernel.TenantID, event string) ([]*WebhookConfig, error)
	Save(ctx context.Context, w *WebhookConfig) error
	Delete(ctx context.Context, id string) error

	// Delivery methods
	SaveDelivery(ctx context.Context, d *WebhookDelivery) error
	UpdateDelivery(ctx context.Context, d *WebhookDelivery) error
	FindPendingDeliveries(ctx context.Context, limit int) ([]*WebhookDelivery, error)
	FindDeliveriesByWebhook(ctx context.Context, webhookID string, limit int) ([]*WebhookDelivery, error)
}
