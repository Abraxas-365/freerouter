package billingcontainer

import (
	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/billing/billingapi"
	"github.com/Abraxas-365/freerouter/internal/billing/billinginfra"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Repo     billing.BillingRepository
	Service  *billingsrv.BillingService
	Handlers *billingapi.BillingHandlers
}

func New(db *sqlx.DB) *Container {
	repo := billinginfra.NewPostgresBillingRepository(db)
	spendingRepo := billinginfra.NewPostgresSpendingLimitRepository(db)

	service := billingsrv.NewBillingService(repo, spendingRepo)
	handlers := billingapi.NewBillingHandlers(service)

	return &Container{
		Repo:     repo,
		Service:  service,
		Handlers: handlers,
	}
}
