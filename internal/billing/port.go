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

	// HasTransactionWithReference reports whether a transaction with the given
	// reference ID already exists (used for payment idempotency).
	HasTransactionWithReference(ctx context.Context, referenceID string) (bool, error)
}

// SpendingLimitRepository defines the contract for spending limit persistence.
type SpendingLimitRepository interface {
	GetByTenantID(ctx context.Context, tenantID kernel.TenantID) (*SpendingLimitConfig, error)
	Upsert(ctx context.Context, cfg *SpendingLimitConfig) (*SpendingLimitConfig, error)
	Delete(ctx context.Context, tenantID kernel.TenantID) error
	// GetDailySpend returns the total spend for a tenant today (UTC).
	GetDailySpend(ctx context.Context, tenantID kernel.TenantID) (float64, error)
	// GetMonthlySpend returns the total spend for a tenant this month (UTC).
	GetMonthlySpend(ctx context.Context, tenantID kernel.TenantID) (float64, error)
}
