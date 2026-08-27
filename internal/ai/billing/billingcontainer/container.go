package billingcontainer

import (
	"github.com/Abraxas-365/freerouter/internal/ai/billing"
	"github.com/Abraxas-365/freerouter/internal/ai/billing/billingapi"
	"github.com/Abraxas-365/freerouter/internal/ai/billing/billinginfra"
	"github.com/Abraxas-365/freerouter/internal/ai/billing/billingsrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Repo     billing.BillingRepository
	Service  *billingsrv.BillingService
	Handlers *billingapi.BillingHandlers
}

func New(db *sqlx.DB) *Container {
	repo := billinginfra.NewPostgresBillingRepository(db)

	service := billingsrv.NewBillingService(repo)
	handlers := billingapi.NewBillingHandlers(service)

	return &Container{
		Repo:     repo,
		Service:  service,
		Handlers: handlers,
	}
}
