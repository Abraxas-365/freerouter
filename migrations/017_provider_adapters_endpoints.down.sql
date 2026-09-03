-- Revert provider adapters & new endpoint surfaces

DELETE FROM model_provider_mappings WHERE id IN (
    'map-openai-whisper-1', 'map-openai-gpt-4o-transcribe', 'map-openai-gpt-4o-mini-transcribe',
    'map-openai-tts-1', 'map-openai-tts-1-hd', 'map-openai-gpt-4o-mini-tts',
    'map-openai-omni-moderation',
    'map-cohere-rerank-v3.5', 'map-cohere-command-a', 'map-cohere-embed-v4'
);

DELETE FROM models WHERE id IN (
    'whisper-1', 'gpt-4o-transcribe', 'gpt-4o-mini-transcribe',
    'tts-1', 'tts-1-hd', 'gpt-4o-mini-tts',
    'omni-moderation-latest',
    'rerank-v3.5', 'command-a-03-2025', 'embed-v4.0'
);

DELETE FROM providers WHERE id IN (
    'fireworks', 'perplexity', 'openrouter', 'cerebras', 'deepinfra', 'azure-openai', 'cohere'
);

ALTER TABLE model_provider_mappings
    DROP COLUMN audio_price_per_minute,
    DROP COLUMN speech_price_per_1k_chars,
    DROP COLUMN rerank_price_per_1k,
    DROP COLUMN audio,
    DROP COLUMN speech,
    DROP COLUMN moderation,
    DROP COLUMN rerank;
