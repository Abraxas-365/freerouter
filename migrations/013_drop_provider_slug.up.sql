-- Remove slug column from providers (replaced by protocol + name).
DROP INDEX IF EXISTS idx_providers_slug;
ALTER TABLE providers DROP CONSTRAINT IF EXISTS providers_slug_key;
ALTER TABLE providers DROP COLUMN IF EXISTS slug;
