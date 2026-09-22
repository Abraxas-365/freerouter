ALTER TABLE providers ADD COLUMN IF NOT EXISTS slug TEXT;
UPDATE providers SET slug = lower(replace(name, ' ', '-')) WHERE slug IS NULL;
ALTER TABLE providers ALTER COLUMN slug SET NOT NULL;
ALTER TABLE providers ADD CONSTRAINT providers_slug_key UNIQUE (slug);
CREATE INDEX IF NOT EXISTS idx_providers_slug ON providers (slug);
