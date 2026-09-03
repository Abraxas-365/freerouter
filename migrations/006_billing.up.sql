-- ============================================================================
-- Billing: credit balances and transaction ledger
-- ============================================================================

-- Tenant credit balances (one row per tenant)
CREATE TABLE tenant_balances (
    tenant_id VARCHAR(255) PRIMARY KEY,
    balance DECIMAL NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_balance_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT chk_balance_non_negative CHECK (balance >= 0)
);

-- Credit transaction ledger (append-only)
CREATE TABLE credit_transactions (
    id VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    amount DECIMAL NOT NULL,           -- Positive = credit, negative = debit
    balance_after DECIMAL NOT NULL,    -- Balance after this transaction
    description TEXT NOT NULL DEFAULT '',
    reference_id VARCHAR(255) NOT NULL DEFAULT '',  -- e.g. usage_log ID, stripe payment ID
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_tx_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT chk_tx_type CHECK (type IN ('top_up', 'usage', 'refund', 'adjust'))
);

CREATE INDEX idx_credit_tx_tenant_created ON credit_transactions(tenant_id, created_at DESC);
CREATE INDEX idx_credit_tx_tenant_type ON credit_transactions(tenant_id, type);
CREATE INDEX idx_credit_tx_reference ON credit_transactions(reference_id);

COMMENT ON TABLE tenant_balances IS 'Current USD credit balance per tenant';
COMMENT ON TABLE credit_transactions IS 'Append-only ledger of all credit movements';
COMMENT ON COLUMN credit_transactions.amount IS 'Positive for credits added, negative for debits';
COMMENT ON COLUMN credit_transactions.balance_after IS 'Tenant balance immediately after this transaction';
COMMENT ON COLUMN credit_transactions.reference_id IS 'Links to usage_log ID, Stripe payment ID, etc.';
