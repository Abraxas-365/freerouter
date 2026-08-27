package billing

import (
	"net/http"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
)

// ============================================================================
// Tenant Balance
// ============================================================================

// TenantBalance holds the current credit balance for a tenant
type TenantBalance struct {
	TenantID  kernel.TenantID `db:"tenant_id" json:"tenant_id"`
	Balance   float64         `db:"balance" json:"balance"` // USD credits remaining
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

func (b *TenantBalance) HasSufficientFunds(amount float64) bool {
	return b.Balance >= amount
}

// ============================================================================
// Transaction Entity (credit ledger)
// ============================================================================

// TransactionType categorizes the transaction
type TransactionType string

const (
	TxTypeTopUp    TransactionType = "top_up"    // Credits added (purchase, grant)
	TxTypeUsage    TransactionType = "usage"     // Credits consumed by gateway requests
	TxTypeRefund   TransactionType = "refund"    // Credits returned
	TxTypeAdjust   TransactionType = "adjust"    // Manual admin adjustment
)

// Transaction is a single entry in the credit ledger
type Transaction struct {
	ID          kernel.TransactionID `db:"id" json:"id"`
	TenantID    kernel.TenantID      `db:"tenant_id" json:"tenant_id"`
	Type        TransactionType      `db:"type" json:"type"`
	Amount      float64              `db:"amount" json:"amount"`           // Positive = credit, negative = debit
	BalanceAfter float64             `db:"balance_after" json:"balance_after"` // Balance after this tx
	Description string               `db:"description" json:"description"`
	ReferenceID string               `db:"reference_id" json:"reference_id,omitempty"` // e.g. usage_log ID, stripe payment ID
	CreatedAt   time.Time            `db:"created_at" json:"created_at"`
}

// ============================================================================
// DTOs
// ============================================================================

type BalanceDTO struct {
	TenantID  kernel.TenantID `json:"tenant_id"`
	Balance   float64         `json:"balance"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (b *TenantBalance) ToDTO() BalanceDTO {
	return BalanceDTO{
		TenantID:  b.TenantID,
		Balance:   b.Balance,
		UpdatedAt: b.UpdatedAt,
	}
}

type TransactionDTO struct {
	ID           kernel.TransactionID `json:"id"`
	Type         TransactionType      `json:"type"`
	Amount       float64              `json:"amount"`
	BalanceAfter float64              `json:"balance_after"`
	Description  string               `json:"description"`
	ReferenceID  string               `json:"reference_id,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
}

func (t *Transaction) ToDTO() TransactionDTO {
	return TransactionDTO{
		ID:           t.ID,
		Type:         t.Type,
		Amount:       t.Amount,
		BalanceAfter: t.BalanceAfter,
		Description:  t.Description,
		ReferenceID:  t.ReferenceID,
		CreatedAt:    t.CreatedAt,
	}
}

// ============================================================================
// Request types
// ============================================================================

// TopUpRequest for adding credits to a tenant
type TopUpRequest struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	ReferenceID string  `json:"reference_id,omitempty"`
}

func (r *TopUpRequest) Validate() error {
	if r.Amount <= 0 {
		return errx.Validation("Amount must be positive").WithDetail("field", "amount")
	}
	return nil
}

// AdjustRequest for manual admin adjustments
type AdjustRequest struct {
	Amount      float64 `json:"amount"` // Can be positive or negative
	Description string  `json:"description"`
}

func (r *AdjustRequest) Validate() error {
	if r.Amount == 0 {
		return errx.Validation("Amount cannot be zero").WithDetail("field", "amount")
	}
	if r.Description == "" {
		return errx.Validation("Description is required for adjustments").WithDetail("field", "description")
	}
	return nil
}

// ============================================================================
// Response types
// ============================================================================

type TransactionListResponse struct {
	Transactions []TransactionDTO `json:"transactions"`
	Total        int              `json:"total"`
}

// TransactionQuery for filtering transactions
type TransactionQuery struct {
	TenantID kernel.TenantID `json:"tenant_id"`
	Type     TransactionType `json:"type,omitempty"`
	From     *time.Time      `json:"from,omitempty"`
	To       *time.Time      `json:"to,omitempty"`
	Limit    int             `json:"limit,omitempty"`
	Offset   int             `json:"offset,omitempty"`
}

// ============================================================================
// Errors
// ============================================================================

var ErrRegistry = errx.NewRegistry("BILLING")

var (
	CodeInsufficientBalance = ErrRegistry.Register("INSUFFICIENT_BALANCE", errx.TypeBusiness, http.StatusPaymentRequired, "Insufficient credit balance")
	CodeBalanceNotFound     = ErrRegistry.Register("BALANCE_NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Tenant balance not found")
)

func ErrInsufficientBalance() *errx.Error { return ErrRegistry.New(CodeInsufficientBalance) }
func ErrBalanceNotFound() *errx.Error     { return ErrRegistry.New(CodeBalanceNotFound) }
