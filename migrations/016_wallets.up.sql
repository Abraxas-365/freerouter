-- ============================================================================
-- Wallets: named sub-balances under a tenant.
-- Funds move atomically between tenant_balances and wallets; API keys can be
-- bound to a wallet so their gateway usage debits the wallet instead.
-- ============================================================================

CREATE TABLE wallets (
    id VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    balance DECIMAL NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_wallet_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT chk_wallet_balance_non_negative CHECK (balance >= 0),
    CONSTRAINT uq_wallet_tenant_name UNIQUE (tenant_id, name)
);

CREATE INDEX idx_wallets_tenant ON wallets(tenant_id);

-- Extend the ledger with wallet transfer types
ALTER TABLE credit_transactions DROP CONSTRAINT chk_tx_type;
ALTER TABLE credit_transactions ADD CONSTRAINT chk_tx_type
    CHECK (type IN ('top_up', 'usage', 'refund', 'adjust', 'wallet_fund', 'wallet_withdraw'));

-- Attribute ledger entries to a wallet (transfers and wallet-paid usage)
ALTER TABLE credit_transactions ADD COLUMN wallet_id VARCHAR(255);
ALTER TABLE credit_transactions ADD CONSTRAINT fk_tx_wallet
    FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE SET NULL;
CREATE INDEX idx_credit_tx_wallet ON credit_transactions(wallet_id) WHERE wallet_id IS NOT NULL;

-- Bind API keys to a wallet (NULL = debit tenant main balance, current behavior)
ALTER TABLE api_keys ADD COLUMN wallet_id VARCHAR(255);
ALTER TABLE api_keys ADD CONSTRAINT fk_api_keys_wallet
    FOREIGN KEY (wallet_id) REFERENCES wallets(id) ON DELETE SET NULL;
CREATE INDEX idx_api_keys_wallet ON api_keys(wallet_id) WHERE wallet_id IS NOT NULL;

-- Attribute usage logs to the wallet that paid for them
ALTER TABLE usage_logs ADD COLUMN wallet_id VARCHAR(255);
CREATE INDEX idx_usage_logs_wallet ON usage_logs(wallet_id) WHERE wallet_id IS NOT NULL;

COMMENT ON TABLE wallets IS 'Named sub-balances under a tenant for budget isolation';
COMMENT ON COLUMN api_keys.wallet_id IS 'When set, gateway usage debits this wallet instead of the tenant main balance';
COMMENT ON COLUMN credit_transactions.wallet_id IS 'Wallet involved in this transaction (transfers and wallet-paid usage)';
