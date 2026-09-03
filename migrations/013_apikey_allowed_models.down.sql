DROP INDEX IF EXISTS idx_api_keys_allowed_models;
ALTER TABLE api_keys DROP COLUMN IF EXISTS allowed_models;
