-- Rollback: Remove seeded provider/model data
-- Note: CASCADE on foreign keys means deleting providers removes mappings too.
-- We delete in reverse order for safety.

DELETE FROM model_provider_mappings WHERE id LIKE 'map-%';
DELETE FROM models WHERE family IN ('openai', 'anthropic', 'google', 'mistral', 'deepseek', 'xai', 'meta');
DELETE FROM providers WHERE id IN ('openai', 'anthropic', 'google-ai-studio', 'mistral', 'deepseek', 'xai', 'groq', 'together');
