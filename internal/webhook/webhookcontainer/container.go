package webhookcontainer

import (
	"github.com/Abraxas-365/freerouter/internal/webhook/webhookapi"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhookinfra"
	"github.com/Abraxas-365/freerouter/internal/webhook/webhooksrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Service  *webhooksrv.WebhookService
	Handlers *webhookapi.WebhookHandlers
}

func New(db *sqlx.DB) *Container {
	repo := webhookinfra.NewPostgresWebhookRepository(db)
	service := webhooksrv.NewWebhookService(repo)
	handlers := webhookapi.NewWebhookHandlers(service)

	return &Container{
		Service:  service,
		Handlers: handlers,
	}
}

func (c *Container) Close() {
	if c.Service != nil {
		c.Service.Stop()
	}
}
