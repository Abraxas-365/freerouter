-- 008_provider_keys_oauth.up.sql
-- Add key_type column to support OAuth credentials alongside API keys.
-- OAuth tokens (access + refresh + expiry) are stored encrypted in the
-- existing token_ciphertext column as a JSON blob.

ALTER TABLE provider_keys
    ADD COLUMN key_type TEXT NOT NULL DEFAULT 'api_key'
        CHECK (key_type IN ('api_key', 'oauth'));

CREATE INDEX idx_provider_keys_key_type ON provider_keys (key_type);
