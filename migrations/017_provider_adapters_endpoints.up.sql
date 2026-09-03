-- ============================================================================
-- Provider adapters & new endpoint surfaces
-- New providers, audio/rerank pricing, and modality capability flags
-- ============================================================================

-- New pricing columns for audio and rerank modalities
ALTER TABLE model_provider_mappings
    ADD COLUMN audio_price_per_minute DECIMAL,
    ADD COLUMN speech_price_per_1k_chars DECIMAL,
    ADD COLUMN rerank_price_per_1k DECIMAL;

COMMENT ON COLUMN model_provider_mappings.audio_price_per_minute IS 'Cost per minute of audio transcribed (STT)';
COMMENT ON COLUMN model_provider_mappings.speech_price_per_1k_chars IS 'Cost per 1000 input characters for speech synthesis (TTS)';
COMMENT ON COLUMN model_provider_mappings.rerank_price_per_1k IS 'Cost per 1000 rerank search units';

-- Modality capability flags
ALTER TABLE model_provider_mappings
    ADD COLUMN audio BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN speech BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN moderation BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN rerank BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN model_provider_mappings.audio IS 'Supports audio transcription (STT)';
COMMENT ON COLUMN model_provider_mappings.speech IS 'Supports speech synthesis (TTS)';
COMMENT ON COLUMN model_provider_mappings.moderation IS 'Supports content moderation';
COMMENT ON COLUMN model_provider_mappings.rerank IS 'Supports document reranking';

-- New providers
INSERT INTO providers (id, name, description, website, status, streaming) VALUES
('fireworks',    'Fireworks AI',  'Fast open-source model inference',                  'https://fireworks.ai',   'active', true),
('perplexity',   'Perplexity',    'Search-augmented LLM API',                          'https://perplexity.ai',  'active', true),
('openrouter',   'OpenRouter',    'Unified LLM API aggregator',                        'https://openrouter.ai',  'active', true),
('cerebras',     'Cerebras',      'Ultra-fast inference on wafer-scale hardware',      'https://cerebras.ai',    'active', true),
('deepinfra',    'DeepInfra',     'Serverless open-source model hosting',              'https://deepinfra.com',  'active', true),
('azure-openai', 'Azure OpenAI',  'OpenAI models on Microsoft Azure',                  'https://azure.microsoft.com/products/ai-services/openai-service', 'active', true),
('cohere',       'Cohere',        'Enterprise LLMs, embeddings, and reranking',        'https://cohere.com',     'active', true)
ON CONFLICT (id) DO NOTHING;

-- New models: audio, moderation, rerank
INSERT INTO models (id, name, family, stability, status) VALUES
('whisper-1',                 'Whisper',                    'openai', 'stable', 'active'),
('gpt-4o-transcribe',         'GPT-4o Transcribe',          'openai', 'stable', 'active'),
('gpt-4o-mini-transcribe',    'GPT-4o Mini Transcribe',     'openai', 'stable', 'active'),
('tts-1',                     'TTS-1',                      'openai', 'stable', 'active'),
('tts-1-hd',                  'TTS-1 HD',                   'openai', 'stable', 'active'),
('gpt-4o-mini-tts',           'GPT-4o Mini TTS',            'openai', 'stable', 'active'),
('omni-moderation-latest',    'Omni Moderation',            'openai', 'stable', 'active'),
('rerank-v3.5',               'Cohere Rerank 3.5',          'cohere', 'stable', 'active'),
('command-a-03-2025',         'Cohere Command A',           'cohere', 'stable', 'active'),
('embed-v4.0',                'Cohere Embed v4',            'cohere', 'stable', 'active')
ON CONFLICT (id) DO NOTHING;

-- Mappings for new models
INSERT INTO model_provider_mappings (
    id, model_id, provider_id, external_id,
    input_price, output_price,
    audio_price_per_minute, speech_price_per_1k_chars, rerank_price_per_1k,
    streaming, audio, speech, moderation, rerank,
    stability, status
) VALUES
-- Audio transcription (STT)
('map-openai-whisper-1',              'whisper-1',              'openai', 'whisper-1',              NULL, NULL, 0.006, NULL,  NULL, false, true,  false, false, false, 'stable', 'active'),
('map-openai-gpt-4o-transcribe',      'gpt-4o-transcribe',      'openai', 'gpt-4o-transcribe',      2.50, 10.0, 0.006, NULL,  NULL, false, true,  false, false, false, 'stable', 'active'),
('map-openai-gpt-4o-mini-transcribe', 'gpt-4o-mini-transcribe', 'openai', 'gpt-4o-mini-transcribe', 1.25, 5.00, 0.003, NULL,  NULL, false, true,  false, false, false, 'stable', 'active'),
-- Speech synthesis (TTS)
('map-openai-tts-1',                  'tts-1',                  'openai', 'tts-1',                  NULL, NULL, NULL,  0.015, NULL, false, false, true,  false, false, 'stable', 'active'),
('map-openai-tts-1-hd',               'tts-1-hd',               'openai', 'tts-1-hd',               NULL, NULL, NULL,  0.030, NULL, false, false, true,  false, false, 'stable', 'active'),
('map-openai-gpt-4o-mini-tts',        'gpt-4o-mini-tts',        'openai', 'gpt-4o-mini-tts',        NULL, NULL, NULL,  0.015, NULL, false, false, true,  false, false, 'stable', 'active'),
-- Moderation (free upstream; request_price stays NULL)
('map-openai-omni-moderation',        'omni-moderation-latest', 'openai', 'omni-moderation-latest', NULL, NULL, NULL,  NULL,  NULL, false, false, false, true,  false, 'stable', 'active'),
-- Cohere
('map-cohere-rerank-v3.5',            'rerank-v3.5',            'cohere', 'rerank-v3.5',            NULL, NULL, NULL,  NULL,  2.00, false, false, false, false, true,  'stable', 'active'),
('map-cohere-command-a',              'command-a-03-2025',      'cohere', 'command-a-03-2025',      2.50, 10.0, NULL,  NULL,  NULL, true,  false, false, false, false, 'stable', 'active'),
('map-cohere-embed-v4',               'embed-v4.0',             'cohere', 'embed-v4.0',             0.12, NULL, NULL,  NULL,  NULL, false, false, false, false, false, 'stable', 'active')
ON CONFLICT (id) DO NOTHING;
