-- ============================================================================
-- Provider Keys (BYOK + platform-managed credentials)
-- ============================================================================

CREATE TABLE provider_keys (
    id VARCHAR(255) PRIMARY KEY,
    provider_id VARCHAR(255) NOT NULL,
    tenant_id VARCHAR(255),  -- NULL = platform-managed key

    -- Credential storage (encrypted; plaintext never stored)
    token_ciphertext TEXT NOT NULL,
    token_masked TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    base_url TEXT,  -- Custom endpoint override

    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    managed BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    sort_order INTEGER,  -- Lower = higher priority; NULL = end of queue

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_provider_key_provider FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    CONSTRAINT fk_provider_key_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT chk_provider_key_status CHECK (status IN ('active', 'inactive')),
    -- Managed keys have no tenant; BYOK keys must have one
    CONSTRAINT chk_provider_key_ownership CHECK (
        (managed = true AND tenant_id IS NULL) OR
        (managed = false AND tenant_id IS NOT NULL)
    )
);

CREATE INDEX idx_provider_keys_provider_id ON provider_keys(provider_id);
CREATE INDEX idx_provider_keys_tenant_id ON provider_keys(tenant_id);
CREATE INDEX idx_provider_keys_managed_provider ON provider_keys(managed, provider_id);
CREATE INDEX idx_provider_keys_status ON provider_keys(status);
CREATE INDEX idx_provider_keys_sort ON provider_keys(provider_id, sort_order);

CREATE TRIGGER update_provider_keys_updated_at BEFORE UPDATE ON provider_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE provider_keys IS 'Upstream LLM provider credentials (BYOK and platform-managed)';
COMMENT ON COLUMN provider_keys.managed IS 'true = platform-owned key for credits-mode; false = tenant BYOK';
COMMENT ON COLUMN provider_keys.token_ciphertext IS 'NaCl secretbox encrypted API key';
COMMENT ON COLUMN provider_keys.token_masked IS 'Masked version for display (e.g. sk-ab****efgh)';
COMMENT ON COLUMN provider_keys.token_hash IS 'HMAC-SHA256 fingerprint for audit matching';
COMMENT ON COLUMN provider_keys.sort_order IS 'Priority among provider keys; lowest first, NULL = end';
