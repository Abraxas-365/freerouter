package wallet

import (
	"time"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Wallet Entity
// ============================================================================

// Wallet is a named sub-balance under a tenant. Funds are moved atomically
// between the tenant's main balance and its wallets. API keys can be bound
// to a wallet so their gateway usage debits the wallet instead of the main
// balance, giving hard budget isolation per customer/team/environment.
type Wallet struct {
	ID          kernel.WalletID `db:"id" json:"id"`
	TenantID    kernel.TenantID `db:"tenant_id" json:"tenant_id"`
	Name        string          `db:"name" json:"name"`
	Description string          `db:"description" json:"description"`
	Balance     float64         `db:"balance" json:"balance"` // USD credits remaining
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updated_at"`
}

func (w *Wallet) HasSufficientFunds(amount float64) bool {
	return w.Balance >= amount
}

func (w *Wallet) IsEmpty() bool {
	return w.Balance == 0
}

// ============================================================================
// Transaction types (extend the billing credit ledger)
// ============================================================================

const (
	// TxTypeWalletFund is recorded on the tenant ledger when funds move
	// from the main balance into a wallet (negative amount on main balance).
	TxTypeWalletFund billing.TransactionType = "wallet_fund"
	// TxTypeWalletWithdraw is recorded on the tenant ledger when funds move
	// from a wallet back to the main balance (positive amount on main balance).
	TxTypeWalletWithdraw billing.TransactionType = "wallet_withdraw"
)

// ============================================================================
// DTOs
// ============================================================================

type WalletDTO struct {
	ID          kernel.WalletID `json:"id"`
	TenantID    kernel.TenantID `json:"tenant_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Balance     float64         `json:"balance"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (w *Wallet) ToDTO() WalletDTO {
	return WalletDTO{
		ID:          w.ID,
		TenantID:    w.TenantID,
		Name:        w.Name,
		Description: w.Description,
		Balance:     w.Balance,
		CreatedAt:   w.CreatedAt,
		UpdatedAt:   w.UpdatedAt,
	}
}

// ============================================================================
// Request types
// ============================================================================

// CreateWalletRequest creates a new (empty) wallet for the caller's tenant.
type CreateWalletRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (r *CreateWalletRequest) Validate() error {
	if r.Name == "" {
		return errx.Validation("Name is required").WithDetail("field", "name")
	}
	if len(r.Name) > 100 {
		return errx.Validation("Name must be at most 100 characters").WithDetail("field", "name")
	}
	return nil
}

// UpdateWalletRequest renames a wallet or changes its description.
type UpdateWalletRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (r *UpdateWalletRequest) Validate() error {
	if r.Name != nil && *r.Name == "" {
		return errx.Validation("Name cannot be empty").WithDetail("field", "name")
	}
	if r.Name != nil && len(*r.Name) > 100 {
		return errx.Validation("Name must be at most 100 characters").WithDetail("field", "name")
	}
	return nil
}

// TransferRequest moves funds between the tenant main balance and a wallet.
// Used by both fund (main -> wallet) and withdraw (wallet -> main).
type TransferRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description,omitempty"`
}

func (r *TransferRequest) Validate() error {
	if r.Amount <= 0 {
		return errx.Validation("Amount must be positive").WithDetail("field", "amount")
	}
	return nil
}

// ============================================================================
// Response types
// ============================================================================

type WalletListResponse struct {
	Wallets []WalletDTO `json:"wallets"`
	Total   int         `json:"total"`
}

// TransferResponse is returned after a fund/withdraw operation.
type TransferResponse struct {
	Wallet      WalletDTO          `json:"wallet"`
	MainBalance billing.BalanceDTO `json:"main_balance"`
}
