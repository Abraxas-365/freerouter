-- Add protocol column to providers.
-- Protocol identifies which wire protocol / auth style the gateway uses for this provider.
ALTER TABLE providers
    ADD COLUMN IF NOT EXISTS protocol TEXT NOT NULL DEFAULT 'openai';

-- Backfill existing providers with correct protocol based on slug.
UPDATE providers SET protocol = 'anthropic'  WHERE slug = 'anthropic';
UPDATE providers SET protocol = 'google'     WHERE slug = 'google';
UPDATE providers SET protocol = 'azure'      WHERE slug = 'azure';
UPDATE providers SET protocol = 'cohere'     WHERE slug = 'cohere';
-- All other providers (openai, mistral, deepseek, xai, groq, together, etc.) are OpenAI-compatible.
