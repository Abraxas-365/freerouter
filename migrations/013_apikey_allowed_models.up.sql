-- Per-key model restrictions: only allow specific models for an API key
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS allowed_models TEXT[] DEFAULT '{}';

-- Index for potential filtering queries
CREATE INDEX IF NOT EXISTS idx_api_keys_allowed_models ON api_keys USING GIN (allowed_models);
