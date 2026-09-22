-- 003_provider_keys.up.sql
-- Encrypted upstream API credentials for providers.

CREATE TABLE provider_keys (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id      UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    token_ciphertext TEXT NOT NULL,
    token_masked     TEXT NOT NULL,
    token_hash       TEXT NOT NULL,
    base_url         TEXT,
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive')),
    sort_order       INTEGER,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_provider_keys_provider ON provider_keys (provider_id);
CREATE INDEX idx_provider_keys_status ON provider_keys (status) WHERE status = 'active';

CREATE TRIGGER update_provider_keys_updated_at
    BEFORE UPDATE ON provider_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
