package billingcontainer

import (
	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/billing/billingapi"
	"github.com/Abraxas-365/freerouter/internal/billing/billinginfra"
	"github.com/Abraxas-365/freerouter/internal/billing/billingsrv"
	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Repo     billing.BillingRepository
	Service  *billingsrv.BillingService
	Stripe   *billingsrv.StripeService
	Handlers *billingapi.BillingHandlers
}

func New(db *sqlx.DB, stripeCfg config.StripeConfig) *Container {
	repo := billinginfra.NewPostgresBillingRepository(db)
	spendingRepo := billinginfra.NewPostgresSpendingLimitRepository(db)

	service := billingsrv.NewBillingService(repo, spendingRepo)
	stripeService := billingsrv.NewStripeService(stripeCfg, service, repo)
	handlers := billingapi.NewBillingHandlers(service, stripeService)

	return &Container{
		Repo:     repo,
		Service:  service,
		Stripe:   stripeService,
		Handlers: handlers,
	}
}
