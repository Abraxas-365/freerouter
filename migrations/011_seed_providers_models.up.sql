-- ============================================================================
-- Seed: Providers, Models & Mappings
-- Core providers and their current models with pricing
-- ============================================================================

-- Deterministic UUIDs via uuid_generate_v5(DNS_NAMESPACE, name).
-- Same UUID every run for the same name; safe to re-run with ON CONFLICT.

-- ============================================================================
-- PROVIDERS
-- ============================================================================
INSERT INTO providers (id, name, protocol, description, website, base_url, status, streaming) VALUES
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'),    'OpenAI',           'openai',    'Creator of GPT models',                       'https://openai.com',          'https://api.openai.com/v1',                       'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'Anthropic',        'anthropic', 'Creator of Claude models',                    'https://anthropic.com',       'https://api.anthropic.com/v1',                    'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'),    'Google AI Studio', 'google',    'Google Gemini models via AI Studio',           'https://aistudio.google.com', 'https://generativelanguage.googleapis.com/v1beta', 'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'),   'Mistral',          'openai',    'European AI lab, creator of Mistral models',   'https://mistral.ai',          'https://api.mistral.ai/v1',                       'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek'),  'DeepSeek',         'openai',    'Chinese AI lab focused on efficient models',   'https://deepseek.com',        'https://api.deepseek.com/v1',                     'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'),       'xAI',              'openai',    'Creator of Grok models',                      'https://x.ai',                'https://api.x.ai/v1',                             'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'groq'),      'Groq',             'openai',    'Ultra-fast inference on custom LPU hardware',  'https://groq.com',            'https://api.groq.com/openai/v1',                  'active', true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'together'),  'Together AI',      'openai',    'Open-source model hosting and inference',      'https://together.ai',         'https://api.together.xyz/v1',                     'active', true)
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- MODELS
-- Model name = the identifier users send in API requests (gateway resolves by name)
-- ============================================================================
INSERT INTO models (id, name, family, stability, status) VALUES
-- OpenAI
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4o'),              'gpt-4o',              'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4o-mini'),         'gpt-4o-mini',         'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4.1'),             'gpt-4.1',             'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4.1-mini'),        'gpt-4.1-mini',        'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4.1-nano'),        'gpt-4.1-nano',        'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o1'),                  'o1',                  'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o3'),                  'o3',                  'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o3-mini'),             'o3-mini',             'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o4-mini'),             'o4-mini',             'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4-turbo'),         'gpt-4-turbo',         'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-3.5-turbo'),       'gpt-3.5-turbo',       'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5'),               'gpt-5',               'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5-mini'),          'gpt-5-mini',          'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5-nano'),          'gpt-5-nano',          'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.1'),             'gpt-5.1',             'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.2'),             'gpt-5.2',             'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.4'),             'gpt-5.4',             'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.4-mini'),        'gpt-5.4-mini',        'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.4-nano'),        'gpt-5.4-nano',        'openai',    'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.5'),             'gpt-5.5',             'openai',    'stable', 'active'),
-- Anthropic
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-sonnet-4-5'),   'claude-sonnet-4-5',   'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-sonnet-4-6'),   'claude-sonnet-4-6',   'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-sonnet-5'),     'claude-sonnet-5',     'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-haiku-4-5'),    'claude-haiku-4-5',    'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-5'),     'claude-opus-4-5',     'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-6'),     'claude-opus-4-6',     'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-7'),     'claude-opus-4-7',     'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-8'),     'claude-opus-4-8',     'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-5'),       'claude-opus-5',       'anthropic', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-fable-5'),      'claude-fable-5',      'anthropic', 'stable', 'active'),
-- Google
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-2.5-pro'),         'gemini-2.5-pro',         'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-2.5-flash'),       'gemini-2.5-flash',       'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-2.5-flash-lite'),  'gemini-2.5-flash-lite',  'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.1-pro-preview'), 'gemini-3.1-pro-preview', 'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.5-flash'),       'gemini-3.5-flash',       'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.5-flash-lite'),  'gemini-3.5-flash-lite',  'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.6-flash'),       'gemini-3.6-flash',       'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.7-flash'),       'gemini-3.7-flash',       'google', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-pro-latest'),      'gemini-pro-latest',      'google', 'stable', 'active'),
-- Mistral
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral-large-2512'),    'mistral-large-2512',    'mistral', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral-small-2506'),    'mistral-small-2506',    'mistral', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'ministral-14b-2512'),    'ministral-14b-2512',    'mistral', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'ministral-8b-2512'),     'ministral-8b-2512',     'mistral', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'ministral-3b-2512'),     'ministral-3b-2512',     'mistral', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'codestral-2508'),        'codestral-2508',        'mistral', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'devstral-2512'),         'devstral-2512',         'mistral', 'stable', 'active'),
-- DeepSeek
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek-v4-pro'),              'deepseek-v4-pro',              'deepseek', 'stable',       'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek-v4-flash'),            'deepseek-v4-flash',            'deepseek', 'stable',       'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek-v4-flash-vision-exp'), 'deepseek-v4-flash-vision-exp', 'deepseek', 'experimental', 'active'),
-- xAI
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4'),                      'grok-4',                      'xai', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4-1-fast-reasoning'),     'grok-4-1-fast-reasoning',     'xai', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4-1-fast-non-reasoning'), 'grok-4-1-fast-non-reasoning', 'xai', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4.3'),                    'grok-4.3',                    'xai', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4.5'),                    'grok-4.5',                    'xai', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4.6'),                    'grok-4.6',                    'xai', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-build-0.1'),              'grok-build-0.1',              'xai', 'stable', 'active'),
-- Meta (Llama — hosted on Groq / Together)
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'llama-3.3-70b-instruct'),    'llama-3.3-70b-instruct',    'meta', 'stable', 'active'),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'llama-3.1-70b-instruct'),    'llama-3.1-70b-instruct',    'meta', 'stable', 'active')
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- MODEL-PROVIDER MAPPINGS
-- Pricing is per million tokens. external_id = the provider's model identifier.
-- ============================================================================
INSERT INTO model_provider_mappings (id, model_id, provider_id, external_id, input_price, output_price, cached_input_price, context_size, max_output, streaming, vision, reasoning, tools, json_output) VALUES

-- ── OpenAI ──────────────────────────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-4o'),         uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4o'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-4o',              2.50,   10.00,  1.25,   128000,  16384,  true,  true,  false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-4o-mini'),    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4o-mini'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-4o-mini',         0.15,   0.60,   0.075,  128000,  16384,  true,  true,  false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-4.1'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4.1'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-4.1',             2.00,   8.00,   0.50,   1000000, NULL,   true,  true,  false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-4.1-mini'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4.1-mini'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-4.1-mini',        0.40,   1.60,   0.10,   1000000, NULL,   true,  true,  false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-4.1-nano'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4.1-nano'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-4.1-nano',        0.10,   0.40,   0.025,  1000000, NULL,   true,  true,  false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-o1'),             uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o1'),           uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'o1',                  15.00,  60.00,  7.50,   200000,  NULL,   true,  true,  true,  false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-o3'),             uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o3'),           uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'o3',                  2.00,   8.00,   0.50,   200000,  NULL,   true,  true,  true,  false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-o3-mini'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o3-mini'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'o3-mini',             1.10,   4.40,   0.55,   200000,  NULL,   true,  false, true,  false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-o4-mini'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'o4-mini'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'o4-mini',             1.10,   4.40,   0.275,  200000,  NULL,   true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-4-turbo'),    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-4-turbo'),  uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-4-turbo',         10.00,  30.00,  NULL,   128000,  NULL,   true,  true,  false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-3.5-turbo'),  uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-3.5-turbo'),uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-3.5-turbo',       0.50,   1.50,   NULL,   16385,   NULL,   true,  false, false, true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5'),          uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5',               1.25,   10.00,  0.125,  400000,  128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5-mini'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5-mini'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5-mini',          0.25,   2.00,   0.025,  400000,  128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5-nano'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5-nano'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5-nano',          0.05,   0.40,   0.005,  400000,  128000, true,  false, true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5.1'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.1'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5.1',             1.25,   10.00,  0.125,  400000,  128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5.2'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.2'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5.2',             1.75,   14.00,  0.175,  400000,  128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5.4'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.4'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5.4',             2.50,   15.00,  0.25,   1050000, 128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5.4-mini'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.4-mini'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5.4-mini',        0.75,   4.50,   0.075,  400000,  128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5.4-nano'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.4-nano'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5.4-nano',        0.20,   1.25,   0.02,   400000,  128000, true,  true,  true,  true,  true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gpt-5.5'),        uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gpt-5.5'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'openai'), 'gpt-5.5',             5.00,   30.00,  0.50,   1050000, 128000, true,  true,  true,  true,  true),

-- ── Anthropic ───────────────────────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-sonnet-4-5'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-sonnet-4-5'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-sonnet-4-5-20250929',  3.00,  15.00,  0.30,  200000,  64000,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-sonnet-4-6'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-sonnet-4-6'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-sonnet-4-6',           3.00,  15.00,  0.30,  1000000, 64000,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-sonnet-5'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-sonnet-5'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-sonnet-5',             2.00,  10.00,  0.20,  1000000, 128000, true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-haiku-4-5'),    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-haiku-4-5'),    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-haiku-4-5-20251001',   1.00,  5.00,   0.10,  200000,  64000,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-opus-4-5'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-5'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-opus-4-5-20251101',    5.00,  25.00,  0.50,  200000,  32000,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-opus-4-6'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-6'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-opus-4-6',             5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-opus-4-7'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-7'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-opus-4-7',             5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-opus-4-8'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-4-8'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-opus-4-8',             5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-opus-5'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-opus-5'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-opus-5',               5.00,  25.00,  0.50,  1000000, 128000, true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-claude-fable-5'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'claude-fable-5'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'anthropic'), 'claude-fable-5',              10.00, 50.00,  1.00,  1000000, 128000, true, true, true, true, true),

-- ── Google AI Studio ────────────────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-2.5-pro'),         uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-2.5-pro'),         uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-2.5-pro',         1.25,  10.00, 0.125, 1048000, 65536,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-2.5-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-2.5-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-2.5-flash',       0.30,  2.50,  0.03,  1048000, 65535,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-2.5-flash-lite'),  uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-2.5-flash-lite'),  uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-2.5-flash-lite',  0.10,  0.40,  0.01,  1048000, 65535,  true, true, false,true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-3.1-pro-preview'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.1-pro-preview'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-3.1-pro-preview', 2.00,  12.00, 0.20,  1048000, 65536,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-3.5-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.5-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-3.5-flash',       1.50,  9.00,  0.15,  1048000, 65536,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-3.5-flash-lite'),  uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.5-flash-lite'),  uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-3.5-flash-lite',  0.30,  2.50,  0.03,  1048000, 65536,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-3.6-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.6-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-3.6-flash',       0.75,  3.75,  0.075, 1048000, 65536,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-3.7-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-3.7-flash'),       uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-3.7-flash',       0.75,  3.75,  0.075, 1048000, 65536,  true, true, true, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-gemini-pro-latest'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'gemini-pro-latest'),      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'google'), 'gemini-pro-latest',      2.00,  12.00, 0.20,  1048000, 65536,  true, true, true, true, true),

-- ── Mistral ─────────────────────────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-mistral-large-2512'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral-large-2512'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'mistral-large-2512', 0.50, 1.50, NULL, 262000,  NULL, true, true,  false, false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-mistral-small-2506'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral-small-2506'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'mistral-small-2506', 0.10, 0.30, NULL, 128000,  NULL, true, true,  false, false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-ministral-14b-2512'),uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'ministral-14b-2512'),uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'ministral-14b-2512', 0.20, 0.20, NULL, 262000,  NULL, true, true,  false, false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-ministral-8b-2512'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'ministral-8b-2512'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'ministral-8b-2512',  0.15, 0.15, NULL, 262000,  NULL, true, true,  false, false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-ministral-3b-2512'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'ministral-3b-2512'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'ministral-3b-2512',  0.10, 0.10, NULL, 131000,  NULL, true, true,  false, false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-codestral-2508'),    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'codestral-2508'),    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'codestral-2508',     0.30, 0.90, NULL, 256000,  NULL, true, false, false, false, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-devstral-2512'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'devstral-2512'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'mistral'), 'devstral-2512',      0.40, 2.00, NULL, 262000,  NULL, true, false, false, false, true),

-- ── DeepSeek ────────────────────────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-deepseek-v4-pro'),              uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek-v4-pro'),              uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek'), 'deepseek-ai/DeepSeek-V4-Pro',              0.435, 0.87,  0.003625, 1050000, 393000, true, false, true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-deepseek-v4-flash'),            uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek-v4-flash'),            uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek'), 'deepseek-ai/DeepSeek-V4-Flash',            0.14,  0.28,  0.0028,   1050000, 393000, true, false, true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-deepseek-v4-flash-vision-exp'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek-v4-flash-vision-exp'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'deepseek'), 'deepseek-ai/DeepSeek-V4-Flash-Vision-Exp', 0.14,  0.28,  0.0028,   1050000, 393000, true, true,  true,  true, true),

-- ── xAI (Grok) ──────────────────────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-4'),                      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4'),                      uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-4',                      3.00,  15.00, 0.75, 256000,  256000, true, true,  true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-4-1-fast-reasoning'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4-1-fast-reasoning'),     uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-4-1-fast-reasoning',     0.20,  0.50,  0.05, 2000000, 30000,  true, true,  true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-4-1-fast-non-reasoning'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4-1-fast-non-reasoning'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-4-1-fast-non-reasoning', 0.20,  0.50,  0.05, 2000000, 30000,  true, true,  false, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-4.3'),                    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4.3'),                    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-4.3',                    1.25,  2.50,  0.20, 1000000, NULL,   true, true,  true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-4.5'),                    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4.5'),                    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-4.5',                    2.00,  6.00,  0.30, 500000,  NULL,   true, true,  true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-4.6'),                    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-4.6'),                    uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-4.6',                    2.00,  6.00,  0.50, 500000,  NULL,   true, true,  true,  true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-grok-build-0.1'),              uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'grok-build-0.1'),              uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'xai'), 'grok-build-0.1',              1.00,  2.00,  0.20, 256000,  256000, true, true,  true,  true, true),

-- ── Groq (Llama models on fast inference) ───────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-llama-3.3-70b-groq'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'llama-3.3-70b-instruct'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'groq'), 'llama-3.3-70b-versatile',                       0.13, 0.40, NULL, 128000, NULL, true, false, false, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-llama-3.1-70b-groq'),   uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'llama-3.1-70b-instruct'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'groq'), 'llama-3.1-70b-versatile',                       0.72, 0.72, NULL, 128000, 2048, true, false, false, true, false),

-- ── Together AI (Llama models) ──────────────────────────────────────────
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-llama-3.3-70b-together'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'llama-3.3-70b-instruct'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'together'), 'meta-llama/Llama-3.3-70B-Instruct-Turbo',           0.13, 0.40, NULL, 128000, NULL, true, false, false, true, true),
(uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'map-llama-3.1-70b-together'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'llama-3.1-70b-instruct'), uuid_generate_v5('6ba7b810-9dad-11d1-80b4-00c04fd430c8', 'together'), 'meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo',      0.72, 0.72, NULL, 128000, NULL, true, false, false, true, true)

ON CONFLICT (id) DO NOTHING;
