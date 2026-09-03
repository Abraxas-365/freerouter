package walletcontainer

import (
	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/wallet"
	"github.com/Abraxas-365/freerouter/internal/wallet/walletapi"
	"github.com/Abraxas-365/freerouter/internal/wallet/walletinfra"
	"github.com/Abraxas-365/freerouter/internal/wallet/walletsrv"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Repo     wallet.WalletRepository
	Service  *walletsrv.WalletService
	Handlers *walletapi.WalletHandlers
}

func New(db *sqlx.DB, billingRepo billing.BillingRepository) *Container {
	repo := walletinfra.NewPostgresWalletRepository(db)
	service := walletsrv.NewWalletService(repo, billingRepo)
	handlers := walletapi.NewWalletHandlers(service)

	return &Container{
		Repo:     repo,
		Service:  service,
		Handlers: handlers,
	}
}
