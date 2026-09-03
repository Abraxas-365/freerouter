package walletinfra

import (
	"context"
	"database/sql"
	"time"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/Abraxas-365/freerouter/internal/kernel"
	"github.com/Abraxas-365/freerouter/internal/wallet"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type PostgresWalletRepository struct {
	db *sqlx.DB
}

func NewPostgresWalletRepository(db *sqlx.DB) wallet.WalletRepository {
	return &PostgresWalletRepository{db: db}
}

const walletColumns = `id, tenant_id, name, description, balance, created_at, updated_at`

func (r *PostgresWalletRepository) Create(ctx context.Context, w *wallet.Wallet) (*wallet.Wallet, error) {
	query := `
		INSERT INTO wallets (id, tenant_id, name, description, balance, created_at, updated_at)
		VALUES (:id, :tenant_id, :name, :description, :balance, :created_at, :updated_at)`

	if _, err := r.db.NamedExecContext(ctx, query, w); err != nil {
		if isUniqueViolation(err) {
			return nil, wallet.ErrWalletNameTaken().WithDetail("name", w.Name)
		}
		return nil, errx.Wrap(err, "failed to create wallet", errx.TypeInternal)
	}
	return w, nil
}

func (r *PostgresWalletRepository) GetByID(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) (*wallet.Wallet, error) {
	var w wallet.Wallet
	err := r.db.GetContext(ctx, &w,
		`SELECT `+walletColumns+` FROM wallets WHERE id = $1 AND tenant_id = $2`,
		id.String(), tenantID.String())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, wallet.ErrWalletNotFound().WithDetail("wallet_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to get wallet", errx.TypeInternal)
	}
	return &w, nil
}

func (r *PostgresWalletRepository) ListByTenant(ctx context.Context, tenantID kernel.TenantID) ([]*wallet.Wallet, error) {
	var wallets []wallet.Wallet
	err := r.db.SelectContext(ctx, &wallets,
		`SELECT `+walletColumns+` FROM wallets WHERE tenant_id = $1 ORDER BY created_at ASC`,
		tenantID.String())
	if err != nil {
		return nil, errx.Wrap(err, "failed to list wallets", errx.TypeInternal)
	}
	result := make([]*wallet.Wallet, len(wallets))
	for i := range wallets {
		result[i] = &wallets[i]
	}
	return result, nil
}

func (r *PostgresWalletRepository) Update(ctx context.Context, w *wallet.Wallet) (*wallet.Wallet, error) {
	query := `
		UPDATE wallets
		SET name = $3, description = $4, updated_at = $5
		WHERE id = $1 AND tenant_id = $2
		RETURNING ` + walletColumns

	var updated wallet.Wallet
	err := r.db.GetContext(ctx, &updated, query,
		w.ID.String(), w.TenantID.String(), w.Name, w.Description, time.Now().UTC())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, wallet.ErrWalletNotFound().WithDetail("wallet_id", w.ID.String())
		}
		if isUniqueViolation(err) {
			return nil, wallet.ErrWalletNameTaken().WithDetail("name", w.Name)
		}
		return nil, errx.Wrap(err, "failed to update wallet", errx.TypeInternal)
	}
	return &updated, nil
}

func (r *PostgresWalletRepository) Delete(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM wallets WHERE id = $1 AND tenant_id = $2`,
		id.String(), tenantID.String())
	if err != nil {
		return errx.Wrap(err, "failed to delete wallet", errx.TypeInternal)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return wallet.ErrWalletNotFound().WithDetail("wallet_id", id.String())
	}
	return nil
}

// Fund atomically moves amount from the tenant main balance into the wallet.
// Locks the balance row first (consistent lock order: tenant_balances -> wallets)
// to avoid deadlocks with concurrent transfers.
func (r *PostgresWalletRepository) Fund(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, amount float64, description string) (*wallet.Wallet, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, errx.Wrap(err, "failed to begin transaction", errx.TypeInternal)
	}
	defer func() { _ = tx.Rollback() }() // rollback after commit is a no-op

	now := time.Now().UTC()

	// Debit main balance (chk_balance_non_negative rejects overdraft)
	var mainBalance float64
	err = tx.GetContext(ctx, &mainBalance, `
		UPDATE tenant_balances
		SET balance = balance - $2, updated_at = $3
		WHERE tenant_id = $1
		RETURNING balance`, tenantID.String(), amount, now)
	if err != nil {
		if err == sql.ErrNoRows || isCheckViolation(err) {
			return nil, wallet.ErrInsufficientBalance().WithDetail("amount", amount)
		}
		return nil, errx.Wrap(err, "failed to debit main balance", errx.TypeInternal)
	}

	// Credit wallet
	var w wallet.Wallet
	err = tx.GetContext(ctx, &w, `
		UPDATE wallets
		SET balance = balance + $3, updated_at = $4
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+walletColumns, id.String(), tenantID.String(), amount, now)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, wallet.ErrWalletNotFound().WithDetail("wallet_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to credit wallet", errx.TypeInternal)
	}

	// Ledger entry on the tenant ledger (negative: money left the main balance)
	if err := insertWalletTx(ctx, tx, tenantID, id, "wallet_fund", -amount, mainBalance, description, ""); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, errx.Wrap(err, "failed to commit wallet fund", errx.TypeInternal)
	}
	return &w, nil
}

// Withdraw atomically moves amount from the wallet back to the main balance.
func (r *PostgresWalletRepository) Withdraw(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, amount float64, description string) (*wallet.Wallet, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, errx.Wrap(err, "failed to begin transaction", errx.TypeInternal)
	}
	defer func() { _ = tx.Rollback() }() // rollback after commit is a no-op

	now := time.Now().UTC()

	// Credit main balance first (consistent lock order: tenant_balances -> wallets)
	var mainBalance float64
	err = tx.GetContext(ctx, &mainBalance, `
		UPDATE tenant_balances
		SET balance = balance + $2, updated_at = $3
		WHERE tenant_id = $1
		RETURNING balance`, tenantID.String(), amount, now)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, wallet.ErrWalletNotFound().WithDetail("tenant_id", tenantID.String())
		}
		return nil, errx.Wrap(err, "failed to credit main balance", errx.TypeInternal)
	}

	// Debit wallet (chk_wallet_balance_non_negative rejects overdraft)
	var w wallet.Wallet
	err = tx.GetContext(ctx, &w, `
		UPDATE wallets
		SET balance = balance - $3, updated_at = $4
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+walletColumns, id.String(), tenantID.String(), amount, now)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, wallet.ErrWalletNotFound().WithDetail("wallet_id", id.String())
		}
		if isCheckViolation(err) {
			return nil, wallet.ErrInsufficientFunds().WithDetail("amount", amount)
		}
		return nil, errx.Wrap(err, "failed to debit wallet", errx.TypeInternal)
	}

	// Ledger entry (positive: money returned to the main balance)
	if err := insertWalletTx(ctx, tx, tenantID, id, "wallet_withdraw", amount, mainBalance, description, ""); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, errx.Wrap(err, "failed to commit wallet withdraw", errx.TypeInternal)
	}
	return &w, nil
}

// DebitUsage atomically subtracts a gateway usage cost from the wallet and
// records a usage transaction. balance_after reflects the wallet balance.
func (r *PostgresWalletRepository) DebitUsage(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID, cost float64, usageLogID string) (*wallet.Wallet, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, errx.Wrap(err, "failed to begin transaction", errx.TypeInternal)
	}
	defer func() { _ = tx.Rollback() }() // rollback after commit is a no-op

	now := time.Now().UTC()

	var w wallet.Wallet
	err = tx.GetContext(ctx, &w, `
		UPDATE wallets
		SET balance = GREATEST(balance - $3, 0), updated_at = $4
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+walletColumns, id.String(), tenantID.String(), cost, now)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, wallet.ErrWalletNotFound().WithDetail("wallet_id", id.String())
		}
		return nil, errx.Wrap(err, "failed to debit wallet usage", errx.TypeInternal)
	}

	if err := insertWalletTx(ctx, tx, tenantID, id, "usage", -cost, w.Balance, "API usage (wallet)", usageLogID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, errx.Wrap(err, "failed to commit wallet usage debit", errx.TypeInternal)
	}
	return &w, nil
}

func (r *PostgresWalletRepository) CountBoundAPIKeys(ctx context.Context, tenantID kernel.TenantID, id kernel.WalletID) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM api_keys WHERE wallet_id = $1 AND tenant_id = $2`,
		id.String(), tenantID.String())
	if err != nil {
		return 0, errx.Wrap(err, "failed to count bound API keys", errx.TypeInternal)
	}
	return count, nil
}

// helpers

func insertWalletTx(ctx context.Context, tx *sqlx.Tx, tenantID kernel.TenantID, walletID kernel.WalletID, txType string, amount, balanceAfter float64, description, referenceID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO credit_transactions (id, tenant_id, wallet_id, type, amount, balance_after, description, reference_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		uuid.NewString(), tenantID.String(), walletID.String(), txType, amount, balanceAfter, description, referenceID, time.Now().UTC())
	if err != nil {
		return errx.Wrap(err, "failed to insert wallet transaction", errx.TypeInternal)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23505"
	}
	return false
}

func isCheckViolation(err error) bool {
	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code == "23514"
	}
	return false
}
