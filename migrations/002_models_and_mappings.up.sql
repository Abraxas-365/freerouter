-- 002_models_and_mappings.up.sql
-- Expand providers, add models, model-provider mappings, and model fallbacks.

-- ── Update trigger function (used by all updated_at columns) ────────
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ── Alter providers ─────────────────────────────────────────────────
ALTER TABLE providers ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE providers ADD COLUMN IF NOT EXISTS website TEXT NOT NULL DEFAULT '';
ALTER TABLE providers ADD COLUMN IF NOT EXISTS streaming BOOLEAN NOT NULL DEFAULT FALSE;

-- Replace boolean active with text status
ALTER TABLE providers ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'inactive'));
UPDATE providers SET status = CASE WHEN active THEN 'active' ELSE 'inactive' END;
ALTER TABLE providers DROP COLUMN IF EXISTS active;

-- Drop the old active partial index, replace with status index
DROP INDEX IF EXISTS idx_providers_active;
CREATE INDEX IF NOT EXISTS idx_providers_status ON providers (status) WHERE status = 'active';

CREATE TRIGGER update_providers_updated_at
    BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ── Models ──────────────────────────────────────────────────────────
CREATE TABLE models (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    family      TEXT NOT NULL,
    stability   TEXT NOT NULL DEFAULT 'stable'
        CHECK (stability IN ('stable', 'beta', 'experimental')),
    status      TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive')),
    free        BOOLEAN NOT NULL DEFAULT FALSE,
    released_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_models_family ON models (family);
CREATE INDEX idx_models_status ON models (status) WHERE status = 'active';

CREATE TRIGGER update_models_updated_at
    BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ── Model-Provider Mappings ─────────────────────────────────────────
CREATE TABLE model_provider_mappings (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_id    UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,

    -- Pricing (per million tokens)
    input_price            DECIMAL,
    output_price           DECIMAL,
    cached_input_price     DECIMAL,
    request_price          DECIMAL,
    image_input_price      DECIMAL,

    -- Modality pricing
    audio_price_per_minute    DECIMAL,
    speech_price_per_1k_chars DECIMAL,
    rerank_price_per_1k       DECIMAL,

    -- Limits
    context_size INTEGER,
    max_output   INTEGER,

    -- Capabilities
    streaming  BOOLEAN NOT NULL DEFAULT FALSE,
    vision     BOOLEAN NOT NULL DEFAULT FALSE,
    reasoning  BOOLEAN NOT NULL DEFAULT FALSE,
    tools      BOOLEAN NOT NULL DEFAULT FALSE,
    json_output BOOLEAN NOT NULL DEFAULT FALSE,
    audio      BOOLEAN NOT NULL DEFAULT FALSE,
    speech     BOOLEAN NOT NULL DEFAULT FALSE,
    moderation BOOLEAN NOT NULL DEFAULT FALSE,
    rerank     BOOLEAN NOT NULL DEFAULT FALSE,

    -- Metadata
    region    TEXT,
    stability TEXT NOT NULL DEFAULT 'stable'
        CHECK (stability IN ('stable', 'beta', 'experimental')),
    status    TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (model_id, provider_id, region)
);

CREATE INDEX idx_mappings_model_id ON model_provider_mappings (model_id);
CREATE INDEX idx_mappings_provider_id ON model_provider_mappings (provider_id);

CREATE TRIGGER update_model_provider_mappings_updated_at
    BEFORE UPDATE ON model_provider_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ── Model Fallbacks ─────────────────────────────────────────────────
CREATE TABLE model_fallbacks (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_id          UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    fallback_model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    priority          INTEGER NOT NULL DEFAULT 0,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (model_id, fallback_model_id),
    CHECK (model_id <> fallback_model_id)
);

CREATE INDEX idx_fallbacks_model_id ON model_fallbacks (model_id);
