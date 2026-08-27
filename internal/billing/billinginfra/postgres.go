package billinginfra

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Abraxas-365/freerouter/internal/billing"
	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresBillingRepository struct {
	db *sqlx.DB
}

func NewPostgresBillingRepository(db *sqlx.DB) billing.BillingRepository {
	return &PostgresBillingRepository{db: db}
}

func (r *PostgresBillingRepository) GetBalance(ctx context.Context, tenantID kernel.TenantID) (*billing.TenantBalance, error) {
	query := `SELECT tenant_id, balance, updated_at FROM tenant_balances WHERE tenant_id = $1`

	var b billing.TenantBalance
	err := r.db.GetContext(ctx, &b, query, tenantID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			// Auto-create with zero balance
			return r.initBalance(ctx, tenantID)
		}
		return nil, errx.Wrap(err, "failed to get tenant balance", errx.TypeInternal)
	}
	return &b, nil
}

func (r *PostgresBillingRepository) Credit(ctx context.Context, tenantID kernel.TenantID, amount float64, txType billing.TransactionType, description, referenceID string) (*billing.TenantBalance, *billing.Transaction, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, errx.Wrap(err, "failed to begin transaction", errx.TypeInternal)
	}
	defer tx.Rollback()

	// Upsert balance with row lock
	upsertQuery := `
		INSERT INTO tenant_balances (tenant_id, balance, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id)
		DO UPDATE SET balance = tenant_balances.balance + $2, updated_at = $3
		RETURNING tenant_id, balance, updated_at`

	now := time.Now().UTC()
	var balance billing.TenantBalance
	if err := tx.GetContext(ctx, &balance, upsertQuery, tenantID.String(), amount, now); err != nil {
		return nil, nil, errx.Wrap(err, "failed to credit balance", errx.TypeInternal)
	}

	// Record transaction
	txRecord := billing.Transaction{
		ID:           kernel.NewTransactionID(uuid.NewString()),
		TenantID:     tenantID,
		Type:         txType,
		Amount:       amount,
		BalanceAfter: balance.Balance,
		Description:  description,
		ReferenceID:  referenceID,
		CreatedAt:    now,
	}

	if err := r.insertTransaction(ctx, tx, txRecord); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, errx.Wrap(err, "failed to commit credit", errx.TypeInternal)
	}

	return &balance, &txRecord, nil
}

func (r *PostgresBillingRepository) Debit(ctx context.Context, tenantID kernel.TenantID, amount float64, txType billing.TransactionType, description, referenceID string) (*billing.TenantBalance, *billing.Transaction, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, errx.Wrap(err, "failed to begin transaction", errx.TypeInternal)
	}
	defer tx.Rollback()

	// Lock and debit atomically — allows negative balance so usage is always
	// recorded. The next request will be rejected by the pre-check in the router.
	debitQuery := `
		UPDATE tenant_balances
		SET balance = balance - $2, updated_at = $3
		WHERE tenant_id = $1
		RETURNING tenant_id, balance, updated_at`

	now := time.Now().UTC()
	var balance billing.TenantBalance
	err = tx.GetContext(ctx, &balance, debitQuery, tenantID.String(), amount, now)
	if err != nil {
		if err == sql.ErrNoRows {
			// No balance row — init with negative balance
			initQuery := `
				INSERT INTO tenant_balances (tenant_id, balance, updated_at)
				VALUES ($1, -$2, $3)
				RETURNING tenant_id, balance, updated_at`
			if err := tx.GetContext(ctx, &balance, initQuery, tenantID.String(), amount, now); err != nil {
				return nil, nil, errx.Wrap(err, "failed to init debit balance", errx.TypeInternal)
			}
		} else {
			return nil, nil, errx.Wrap(err, "failed to debit balance", errx.TypeInternal)
		}
	}

	// Record transaction (negative amount for debits)
	txRecord := billing.Transaction{
		ID:           kernel.NewTransactionID(uuid.NewString()),
		TenantID:     tenantID,
		Type:         txType,
		Amount:       -amount,
		BalanceAfter: balance.Balance,
		Description:  description,
		ReferenceID:  referenceID,
		CreatedAt:    now,
	}

	if err := r.insertTransaction(ctx, tx, txRecord); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, errx.Wrap(err, "failed to commit debit", errx.TypeInternal)
	}

	return &balance, &txRecord, nil
}

func (r *PostgresBillingRepository) QueryTransactions(ctx context.Context, q billing.TransactionQuery) ([]*billing.Transaction, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
	args = append(args, q.TenantID.String())
	argIdx++

	if q.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, string(q.Type))
		argIdx++
	}
	if q.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *q.From)
		argIdx++
	}
	if q.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *q.To)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count
	var total int
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM credit_transactions "+where, args...); err != nil {
		return nil, 0, errx.Wrap(err, "failed to count transactions", errx.TypeInternal)
	}

	// Fetch
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, tenant_id, type, amount, balance_after, description, reference_id, created_at
		FROM credit_transactions %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var transactions []billing.Transaction
	if err := r.db.SelectContext(ctx, &transactions, dataQuery, args...); err != nil {
		return nil, 0, errx.Wrap(err, "failed to query transactions", errx.TypeInternal)
	}

	result := make([]*billing.Transaction, len(transactions))
	for i := range transactions {
		result[i] = &transactions[i]
	}
	return result, total, nil
}

// helpers

func (r *PostgresBillingRepository) initBalance(ctx context.Context, tenantID kernel.TenantID) (*billing.TenantBalance, error) {
	query := `
		INSERT INTO tenant_balances (tenant_id, balance, updated_at)
		VALUES ($1, 0, $2)
		ON CONFLICT (tenant_id) DO NOTHING
		RETURNING tenant_id, balance, updated_at`

	now := time.Now().UTC()
	var b billing.TenantBalance
	err := r.db.GetContext(ctx, &b, query, tenantID.String(), now)
	if err != nil {
		if err == sql.ErrNoRows {
			// Race: another goroutine created it, re-read
			return r.GetBalance(ctx, tenantID)
		}
		return nil, errx.Wrap(err, "failed to init tenant balance", errx.TypeInternal)
	}
	return &b, nil
}

func (r *PostgresBillingRepository) insertTransaction(ctx context.Context, tx *sqlx.Tx, t billing.Transaction) error {
	query := `
		INSERT INTO credit_transactions (id, tenant_id, type, amount, balance_after, description, reference_id, created_at)
		VALUES (:id, :tenant_id, :type, :amount, :balance_after, :description, :reference_id, :created_at)`

	_, err := tx.NamedExecContext(ctx, query, t)
	if err != nil {
		return errx.Wrap(err, "failed to insert transaction", errx.TypeInternal)
	}
	return nil
}
