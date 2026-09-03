package wallet

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// WalletRepository defines the contract for wallet persistence.
// Fund/Withdraw move money between the tenant main balance and a wallet
// and MUST be atomic: both the wallet row, the tenant_balances row, and
// the credit_transactions ledger entry commit together or not at all.
type WalletRepository interface {
	// Create inserts a new wallet with zero balance.
	// Returns ErrWalletNameTaken if the tenant already has a wallet with the same name.
	Create(ctx context.Context, w *Wallet) (*Wallet, error)

	// GetByID returns a wallet scoped to the tenant.
	// Returns ErrWalletNotFound if it does not exist or belongs to another tenant.
	GetByID(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) (*Wallet, error)

	// ListByTenant returns all wallets for a tenant.
	ListByTenant(ctx context.Context, tenantID kernel.TenantID) ([]*Wallet, error)

	// Update changes name/description (not balance).
	Update(ctx context.Context, w *Wallet) (*Wallet, error)

	// Delete removes a wallet. Callers must ensure it is empty and unbound first.
	Delete(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) error

	// Fund atomically moves amount from the tenant main balance into the wallet
	// and records a wallet_fund transaction on the ledger.
	// Returns ErrInsufficientBalance if the main balance would go negative.
	Fund(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, amount float64, description string) (*Wallet, error)

	// Withdraw atomically moves amount from the wallet back to the tenant main
	// balance and records a wallet_withdraw transaction on the ledger.
	// Returns ErrInsufficientFunds if the wallet balance would go negative.
	Withdraw(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, amount float64, description string) (*Wallet, error)

	// DebitUsage atomically subtracts a gateway usage cost from the wallet and
	// records a usage transaction referencing the usage log.
	// Returns ErrInsufficientFunds if the wallet balance would go negative.
	DebitUsage(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, cost float64, usageLogID string) (*Wallet, error)

	// CountBoundAPIKeys returns how many API keys reference this wallet.
	// Used to block deletion of wallets that are still in use.
	CountBoundAPIKeys(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) (int, error)
}
