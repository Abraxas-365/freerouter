package usagecontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usageapi"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usageinfra"
	"github.com/Abraxas-365/freerouter/internal/ai/usage/usagesrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Service  *usagesrv.UsageService
	Handlers *usageapi.UsageHandlers
}

func New(db *sqlx.DB, bufferSize int) *Container {
	repo := usageinfra.NewPostgresUsageRepository(db)

	service := usagesrv.NewUsageService(repo, bufferSize)
	handlers := usageapi.NewUsageHandlers(service)

	return &Container{
		Service:  service,
		Handlers: handlers,
	}
}

func (c *Container) Close() {
	c.Service.Close()
}
