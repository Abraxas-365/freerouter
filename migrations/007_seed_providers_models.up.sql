-- ============================================================================
-- Seed: Providers, Models & Mappings
-- Core providers and their current models with pricing
-- ============================================================================

-- ============================================================================
-- PROVIDERS
-- ============================================================================
INSERT INTO providers (id, name, description, website, status, streaming) VALUES
('openai',           'OpenAI',           'Creator of GPT models',                          'https://openai.com',                    'active', true),
('anthropic',        'Anthropic',        'Creator of Claude models',                       'https://anthropic.com',                 'active', true),
('google-ai-studio', 'Google AI Studio', 'Google Gemini models via AI Studio',              'https://aistudio.google.com',           'active', true),
('mistral',          'Mistral',          'European AI lab, creator of Mistral models',     'https://mistral.ai',                    'active', true),
('deepseek',         'DeepSeek',         'Chinese AI lab focused on efficient models',     'https://deepseek.com',                  'active', true),
('xai',              'xAI',              'Creator of Grok models',                         'https://x.ai',                          'active', true),
('groq',             'Groq',             'Ultra-fast inference on custom LPU hardware',    'https://groq.com',                      'active', true),
('together',         'Together AI',      'Open-source model hosting and inference',        'https://together.ai',                   'active', true)
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- MODELS
-- ============================================================================
INSERT INTO models (id, name, family, stability, status) VALUES
-- OpenAI
('gpt-4o',              'GPT-4o',              'openai',    'stable', 'active'),
('gpt-4o-mini',         'GPT-4o Mini',         'openai',    'stable', 'active'),
('gpt-4.1',             'GPT-4.1',             'openai',    'stable', 'active'),
('gpt-4.1-mini',        'GPT-4.1 Mini',        'openai',    'stable', 'active'),
('gpt-4.1-nano',        'GPT-4.1 Nano',        'openai',    'stable', 'active'),
('o1',                  'o1',                  'openai',    'stable', 'active'),
('o3',                  'o3',                  'openai',    'stable', 'active'),
('o3-mini',             'o3 Mini',             'openai',    'stable', 'active'),
('o4-mini',             'o4 Mini',             'openai',    'stable', 'active'),
('gpt-4-turbo',         'GPT-4 Turbo',         'openai',    'stable', 'active'),
('gpt-3.5-turbo',       'GPT-3.5 Turbo',       'openai',    'stable', 'active'),
('gpt-5',               'GPT-5',               'openai',    'stable', 'active'),
('gpt-5-mini',          'GPT-5 Mini',          'openai',    'stable', 'active'),
('gpt-5-nano',          'GPT-5 Nano',          'openai',    'stable', 'active'),
('gpt-5.1',             'GPT-5.1',             'openai',    'stable', 'active'),
('gpt-5.2',             'GPT-5.2',             'openai',    'stable', 'active'),
('gpt-5.4',             'GPT-5.4',             'openai',    'stable', 'active'),
('gpt-5.4-mini',        'GPT-5.4 Mini',        'openai',    'stable', 'active'),
('gpt-5.4-nano',        'GPT-5.4 Nano',        'openai',    'stable', 'active'),
('gpt-5.5',             'GPT-5.5',             'openai',    'stable', 'active'),
-- Anthropic
('claude-sonnet-4-5',   'Claude Sonnet 4.5',   'anthropic', 'stable', 'active'),
('claude-sonnet-4-6',   'Claude Sonnet 4.6',   'anthropic', 'stable', 'active'),
('claude-sonnet-5',     'Claude Sonnet 5',     'anthropic', 'stable', 'active'),
('claude-haiku-4-5',    'Claude Haiku 4.5',    'anthropic', 'stable', 'active'),
('claude-opus-4-5',     'Claude Opus 4.5',     'anthropic', 'stable', 'active'),
('claude-opus-4-6',     'Claude Opus 4.6',     'anthropic', 'stable', 'active'),
('claude-opus-4-7',     'Claude Opus 4.7',     'anthropic', 'stable', 'active'),
('claude-opus-4-8',     'Claude Opus 4.8',     'anthropic', 'stable', 'active'),
('claude-opus-5',       'Claude Opus 5',       'anthropic', 'stable', 'active'),
('claude-fable-5',      'Claude Fable 5',      'anthropic', 'stable', 'active'),
-- Google
('gemini-2.5-pro',         'Gemini 2.5 Pro',         'google', 'stable', 'active'),
('gemini-2.5-flash',       'Gemini 2.5 Flash',       'google', 'stable', 'active'),
('gemini-2.5-flash-lite',  'Gemini 2.5 Flash Lite',  'google', 'stable', 'active'),
('gemini-3.1-pro-preview', 'Gemini 3.1 Pro Preview', 'google', 'stable', 'active'),
('gemini-3.5-flash',       'Gemini 3.5 Flash',       'google', 'stable', 'active'),
('gemini-3.5-flash-lite',  'Gemini 3.5 Flash Lite',  'google', 'stable', 'active'),
('gemini-3.6-flash',       'Gemini 3.6 Flash',       'google', 'stable', 'active'),
('gemini-3.7-flash',       'Gemini 3.7 Flash',       'google', 'stable', 'active'),
('gemini-pro-latest',      'Gemini Pro Latest',      'google', 'stable', 'active'),
-- Mistral
('mistral-large-2512',    'Mistral Large 25.12',      'mistral', 'stable', 'active'),
('mistral-small-2506',    'Mistral Small 25.06',      'mistral', 'stable', 'active'),
('ministral-14b-2512',    'Ministral 14B 25.12',      'mistral', 'stable', 'active'),
('ministral-8b-2512',     'Ministral 8B 25.12',       'mistral', 'stable', 'active'),
('ministral-3b-2512',     'Ministral 3B 25.12',       'mistral', 'stable', 'active'),
('codestral-2508',        'Codestral 25.08',          'mistral', 'stable', 'active'),
('devstral-2512',         'Devstral 25.12',           'mistral', 'stable', 'active'),
-- DeepSeek
('deepseek-v4-pro',              'DeepSeek V4 Pro',         'deepseek', 'stable', 'active'),
('deepseek-v4-flash',            'DeepSeek V4 Flash',       'deepseek', 'stable', 'active'),
('deepseek-v4-flash-vision-exp', 'DeepSeek V4 Flash Vision','deepseek', 'experimental', 'active'),
-- xAI
('grok-4',                    'Grok 4',                    'xai', 'stable', 'active'),
('grok-4-1-fast-reasoning',  'Grok 4.1 Fast Reasoning',   'xai', 'stable', 'active'),
('grok-4-1-fast-non-reasoning','Grok 4.1 Fast Non-Reasoning','xai','stable','active'),
('grok-4.3',                  'Grok 4.3',                  'xai', 'stable', 'active'),
('grok-4.5',                  'Grok 4.5',                  'xai', 'stable', 'active'),
('grok-4.6',                  'Grok 4.6',                  'xai', 'stable', 'active'),
('grok-build-0.1',            'Grok Build 0.1',            'xai', 'stable', 'active'),
-- Meta
('llama-3.3-70b-instruct',    'Llama 3.3 70B Instruct',   'meta', 'stable', 'active'),
('llama-3.1-70b-instruct',    'Llama 3.1 70B Instruct',   'meta', 'stable', 'active')
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- MODEL-PROVIDER MAPPINGS
-- ============================================================================

-- Helper: UUIDs for mapping IDs use md5 of model_id + provider_id for determinism
-- We use human-readable IDs instead for simplicity in seed data.

INSERT INTO model_provider_mappings (id, model_id, provider_id, external_id, input_price, output_price, cached_input_price, context_size, max_output, streaming, vision, reasoning, tools, json_output) VALUES

-- ============================================================================
-- OpenAI
-- ============================================================================
('map-gpt-4o',              'gpt-4o',        'openai', 'gpt-4o',              2.50,   10.00,  1.25,   128000,  16384,  true,  true,  false, true,  true),
('map-gpt-4o-mini',         'gpt-4o-mini',   'openai', 'gpt-4o-mini',         0.15,   0.60,   0.075,  128000,  16384,  true,  true,  false, true,  true),
('map-gpt-4.1',             'gpt-4.1',       'openai', 'gpt-4.1',             2.00,   8.00,   0.50,   1000000, NULL,   true,  true,  false, true,  true),
('map-gpt-4.1-mini',        'gpt-4.1-mini',  'openai', 'gpt-4.1-mini',        0.40,   1.60,   0.10,   1000000, NULL,   true,  true,  false, true,  true),
('map-gpt-4.1-nano',        'gpt-4.1-nano',  'openai', 'gpt-4.1-nano',        0.10,   0.40,   0.025,  1000000, NULL,   true,  true,  false, true,  true),
('map-o1',                  'o1',            'openai', 'o1',                  15.00,  60.00,  7.50,   200000,  NULL,   true,  true,  true,  false, true),
('map-o3',                  'o3',            'openai', 'o3',                  2.00,   8.00,   0.50,   200000,  NULL,   true,  true,  true,  false, true),
('map-o3-mini',             'o3-mini',       'openai', 'o3-mini',             1.10,   4.40,   0.55,   200000,  NULL,   true,  false, true,  false, true),
('map-o4-mini',             'o4-mini',       'openai', 'o4-mini',             1.10,   4.40,   0.275,  200000,  NULL,   true,  true,  true,  true,  true),
('map-gpt-4-turbo',         'gpt-4-turbo',   'openai', 'gpt-4-turbo',         10.00,  30.00,  NULL,   128000,  NULL,   true,  true,  false, true,  true),
('map-gpt-3.5-turbo',       'gpt-3.5-turbo', 'openai', 'gpt-3.5-turbo',       0.50,   1.50,   NULL,   16385,   NULL,   true,  false, false, true,  true),
('map-gpt-5',               'gpt-5',         'openai', 'gpt-5',               1.25,   10.00,  0.125,  400000,  128000, true,  true,  true,  true,  true),
('map-gpt-5-mini',          'gpt-5-mini',    'openai', 'gpt-5-mini',          0.25,   2.00,   0.025,  400000,  128000, true,  true,  true,  true,  true),
('map-gpt-5-nano',          'gpt-5-nano',    'openai', 'gpt-5-nano',          0.05,   0.40,   0.005,  400000,  128000, true,  false, true,  true,  true),
('map-gpt-5.1',             'gpt-5.1',       'openai', 'gpt-5.1',             1.25,   10.00,  0.125,  400000,  128000, true,  true,  true,  true,  true),
('map-gpt-5.2',             'gpt-5.2',       'openai', 'gpt-5.2',             1.75,   14.00,  0.175,  400000,  128000, true,  true,  true,  true,  true),
('map-gpt-5.4',             'gpt-5.4',       'openai', 'gpt-5.4',             2.50,   15.00,  0.25,   1050000, 128000, true,  true,  true,  true,  true),
('map-gpt-5.4-mini',        'gpt-5.4-mini',  'openai', 'gpt-5.4-mini',        0.75,   4.50,   0.075,  400000,  128000, true,  true,  true,  true,  true),
('map-gpt-5.4-nano',        'gpt-5.4-nano',  'openai', 'gpt-5.4-nano',        0.20,   1.25,   0.02,   400000,  128000, true,  true,  true,  true,  true),
('map-gpt-5.5',             'gpt-5.5',       'openai', 'gpt-5.5',             5.00,   30.00,  0.50,   1050000, 128000, true,  true,  true,  true,  true),

-- ============================================================================
-- Anthropic
-- ============================================================================
('map-claude-sonnet-4-5',   'claude-sonnet-4-5',   'anthropic', 'claude-sonnet-4-5-20250929',  3.00,  15.00,  0.30,  200000,  64000,  true, true, true, true, true),
('map-claude-sonnet-4-6',   'claude-sonnet-4-6',   'anthropic', 'claude-sonnet-4-6',           3.00,  15.00,  0.30,  1000000, 64000,  true, true, true, true, true),
('map-claude-sonnet-5',     'claude-sonnet-5',     'anthropic', 'claude-sonnet-5',             2.00,  10.00,  0.20,  1000000, 128000, true, true, true, true, true),
('map-claude-haiku-4-5',    'claude-haiku-4-5',    'anthropic', 'claude-haiku-4-5-20251001',   1.00,  5.00,   0.10,  200000,  64000,  true, true, true, true, true),
('map-claude-opus-4-5',     'claude-opus-4-5',     'anthropic', 'claude-opus-4-5-20251101',    5.00,  25.00,  0.50,  200000,  32000,  true, true, true, true, true),
('map-claude-opus-4-6',     'claude-opus-4-6',     'anthropic', 'claude-opus-4-6',             5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
('map-claude-opus-4-7',     'claude-opus-4-7',     'anthropic', 'claude-opus-4-7',             5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
('map-claude-opus-4-8',     'claude-opus-4-8',     'anthropic', 'claude-opus-4-8',             5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
('map-claude-opus-5',       'claude-opus-5',       'anthropic', 'claude-opus-5',               5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
('map-claude-fable-5',      'claude-fable-5',      'anthropic', 'claude-fable-5',              10.00, 50.00,  1.00,  1000000, 128000, true, true, true, true, true),

-- ============================================================================
-- Google AI Studio
-- ============================================================================
('map-gemini-2.5-pro',         'gemini-2.5-pro',         'google-ai-studio', 'gemini-2.5-pro',         1.25,  10.00, 0.125, 1048000, 65536,  true, true, true, true, true),
('map-gemini-2.5-flash',       'gemini-2.5-flash',       'google-ai-studio', 'gemini-2.5-flash',       0.30,  2.50,  0.03,  1048000, 65535,  true, true, true, true, true),
('map-gemini-2.5-flash-lite',  'gemini-2.5-flash-lite',  'google-ai-studio', 'gemini-2.5-flash-lite',  0.10,  0.40,  0.01,  1048000, 65535,  true, true, false,true, true),
('map-gemini-3.1-pro-preview', 'gemini-3.1-pro-preview', 'google-ai-studio', 'gemini-3.1-pro-preview', 2.00,  12.00, 0.20,  1048000, 65536,  true, true, true, true, true),
('map-gemini-3.5-flash',       'gemini-3.5-flash',       'google-ai-studio', 'gemini-3.5-flash',       1.50,  9.00,  0.15,  1048000, 65536,  true, true, true, true, true),
('map-gemini-3.5-flash-lite',  'gemini-3.5-flash-lite',  'google-ai-studio', 'gemini-3.5-flash-lite',  0.30,  2.50,  0.03,  1048000, 65536,  true, true, true, true, true),
('map-gemini-3.6-flash',       'gemini-3.6-flash',       'google-ai-studio', 'gemini-3.6-flash',       0.75,  3.75,  0.075, 1048000, 65536,  true, true, true, true, true),
('map-gemini-3.7-flash',       'gemini-3.7-flash',       'google-ai-studio', 'gemini-3.7-flash',       0.75,  3.75,  0.075, 1048000, 65536,  true, true, true, true, true),
('map-gemini-pro-latest',      'gemini-pro-latest',      'google-ai-studio', 'gemini-pro-latest',      2.00,  12.00, 0.20,  1048000, 65536,  true, true, true, true, true),

-- ============================================================================
-- Mistral
-- ============================================================================
('map-mistral-large-2512',    'mistral-large-2512',  'mistral', 'mistral-large-2512',    0.50,  1.50,  NULL, 262000,  NULL, true, true,  false, false, true),
('map-mistral-small-2506',    'mistral-small-2506',  'mistral', 'mistral-small-2506',    0.10,  0.30,  NULL, 128000,  NULL, true, true,  false, false, true),
('map-ministral-14b-2512',    'ministral-14b-2512',  'mistral', 'ministral-14b-2512',    0.20,  0.20,  NULL, 262000,  NULL, true, true,  false, false, true),
('map-ministral-8b-2512',     'ministral-8b-2512',   'mistral', 'ministral-8b-2512',     0.15,  0.15,  NULL, 262000,  NULL, true, true,  false, false, true),
('map-ministral-3b-2512',     'ministral-3b-2512',   'mistral', 'ministral-3b-2512',     0.10,  0.10,  NULL, 131000,  NULL, true, true,  false, false, true),
('map-codestral-2508',        'codestral-2508',      'mistral', 'codestral-2508',        0.30,  0.90,  NULL, 256000,  NULL, true, false, false, false, true),
('map-devstral-2512',         'devstral-2512',       'mistral', 'devstral-2512',         0.40,  2.00,  NULL, 262000,  NULL, true, false, false, false, true),

-- ============================================================================
-- DeepSeek
-- ============================================================================
('map-deepseek-v4-pro',              'deepseek-v4-pro',              'deepseek', 'deepseek-ai/DeepSeek-V4-Pro',              0.435, 0.87,  0.003625, 1050000, 393000, true, false, true,  true, true),
('map-deepseek-v4-flash',            'deepseek-v4-flash',            'deepseek', 'deepseek-ai/DeepSeek-V4-Flash',            0.14,  0.28,  0.0028,   1050000, 393000, true, false, true,  true, true),
('map-deepseek-v4-flash-vision-exp', 'deepseek-v4-flash-vision-exp', 'deepseek', 'deepseek-ai/DeepSeek-V4-Flash-Vision-Exp', 0.14,  0.28,  0.0028,   1050000, 393000, true, true,  true,  true, true),

-- ============================================================================
-- xAI (Grok)
-- ============================================================================
('map-grok-4',                     'grok-4',                     'xai', 'grok-4',                     3.00,  15.00, 0.75, 256000,  256000, true, true,  true,  true, true),
('map-grok-4-1-fast-reasoning',    'grok-4-1-fast-reasoning',    'xai', 'grok-4-1-fast-reasoning',    0.20,  0.50,  0.05, 2000000, 30000,  true, true,  true,  true, true),
('map-grok-4-1-fast-non-reasoning','grok-4-1-fast-non-reasoning','xai', 'grok-4-1-fast-non-reasoning',0.20,  0.50,  0.05, 2000000, 30000,  true, true,  false, true, true),
('map-grok-4.3',                   'grok-4.3',                   'xai', 'grok-4.3',                   1.25,  2.50,  0.20, 1000000, NULL,   true, true,  true,  true, true),
('map-grok-4.5',                   'grok-4.5',                   'xai', 'grok-4.5',                   2.00,  6.00,  0.30, 500000,  NULL,   true, true,  true,  true, true),
('map-grok-4.6',                   'grok-4.6',                   'xai', 'grok-4.6',                   2.00,  6.00,  0.50, 500000,  NULL,   true, true,  true,  true, true),
('map-grok-build-0.1',             'grok-build-0.1',             'xai', 'grok-build-0.1',             1.00,  2.00,  0.20, 256000,  256000, true, true,  true,  true, true),

-- ============================================================================
-- Groq (Llama models on fast inference)
-- ============================================================================
('map-llama-3.3-70b-groq',   'llama-3.3-70b-instruct', 'groq', 'llama-3.3-70b-versatile', 0.13, 0.40, NULL, 128000, NULL, true, false, false, true, true),
('map-llama-3.1-70b-groq',   'llama-3.1-70b-instruct', 'groq', 'llama-3.1-70b-versatile', 0.72, 0.72, NULL, 128000, 2048, true, false, false, true, false),

-- ============================================================================
-- Together AI (Llama models)
-- ============================================================================
('map-llama-3.3-70b-together',   'llama-3.3-70b-instruct', 'together', 'meta-llama/Llama-3.3-70B-Instruct-Turbo', 0.13, 0.40, NULL, 128000, NULL, true, false, false, true, true),
('map-llama-3.1-70b-together',   'llama-3.1-70b-instruct', 'together', 'meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo', 0.72, 0.72, NULL, 128000, NULL, true, false, false, true, true)

ON CONFLICT (id) DO NOTHING;
