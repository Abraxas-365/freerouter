package billing

import (
	"context"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// BillingRepository defines the contract for billing persistence.
// Balance mutations and transaction inserts must be atomic.
type BillingRepository interface {
	// GetBalance returns the current balance for a tenant (creates with 0 if missing)
	GetBalance(ctx context.Context, tenantID kernel.TenantID) (*TenantBalance, error)

	// Credit atomically adds amount to balance and records a transaction.
	// Returns the new balance and the created transaction.
	Credit(ctx context.Context, tenantID kernel.TenantID, amount float64, txType TransactionType, description, referenceID string) (*TenantBalance, *Transaction, error)

	// Debit atomically subtracts amount from balance and records a transaction.
	// Returns ErrInsufficientBalance if balance would go negative.
	Debit(ctx context.Context, tenantID kernel.TenantID, amount float64, txType TransactionType, description, referenceID string) (*TenantBalance, *Transaction, error)

	// QueryTransactions returns paginated transactions for a tenant
	QueryTransactions(ctx context.Context, q TransactionQuery) ([]*Transaction, int, error)
}
