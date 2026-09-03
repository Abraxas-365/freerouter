-- Revert wallets

ALTER TABLE usage_logs DROP COLUMN IF EXISTS wallet_id;

ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS fk_api_keys_wallet;
ALTER TABLE api_keys DROP COLUMN IF EXISTS wallet_id;

ALTER TABLE credit_transactions DROP CONSTRAINT IF EXISTS fk_tx_wallet;
ALTER TABLE credit_transactions DROP COLUMN IF EXISTS wallet_id;

ALTER TABLE credit_transactions DROP CONSTRAINT chk_tx_type;
ALTER TABLE credit_transactions ADD CONSTRAINT chk_tx_type
    CHECK (type IN ('top_up', 'usage', 'refund', 'adjust'));

DROP TABLE IF EXISTS wallets;
