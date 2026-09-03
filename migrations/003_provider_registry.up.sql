-- ============================================================================
-- Provider & Model Registry
-- ============================================================================

-- Providers (e.g. OpenAI, Anthropic, Google)
CREATE TABLE providers (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    website TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    streaming BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_provider_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_providers_status ON providers(status);

-- Models (e.g. gpt-4o, claude-sonnet-4-20250514)
CREATE TABLE models (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    family VARCHAR(255) NOT NULL,
    stability VARCHAR(50) NOT NULL DEFAULT 'stable',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    free BOOLEAN NOT NULL DEFAULT FALSE,
    released_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chk_model_stability CHECK (stability IN ('stable', 'beta', 'experimental')),
    CONSTRAINT chk_model_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_models_status ON models(status);
CREATE INDEX idx_models_family ON models(family);

-- Model-Provider Mappings (links models to providers with pricing and capabilities)
CREATE TABLE model_provider_mappings (
    id VARCHAR(255) PRIMARY KEY,
    model_id VARCHAR(255) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    external_id VARCHAR(255) NOT NULL,

    -- Pricing (per million tokens)
    input_price DECIMAL,
    output_price DECIMAL,
    cached_input_price DECIMAL,
    request_price DECIMAL,
    image_input_price DECIMAL,

    -- Limits
    context_size INTEGER,
    max_output INTEGER,

    -- Capabilities
    streaming BOOLEAN NOT NULL DEFAULT FALSE,
    vision BOOLEAN NOT NULL DEFAULT FALSE,
    reasoning BOOLEAN NOT NULL DEFAULT FALSE,
    tools BOOLEAN NOT NULL DEFAULT FALSE,
    json_output BOOLEAN NOT NULL DEFAULT FALSE,

    region VARCHAR(100),
    stability VARCHAR(50) NOT NULL DEFAULT 'stable',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_mapping_model FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE CASCADE,
    CONSTRAINT fk_mapping_provider FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE,
    CONSTRAINT uq_mapping_model_provider_region UNIQUE (model_id, provider_id, region),
    CONSTRAINT chk_mapping_stability CHECK (stability IN ('stable', 'beta', 'experimental')),
    CONSTRAINT chk_mapping_status CHECK (status IN ('active', 'inactive'))
);

CREATE INDEX idx_mappings_model_id ON model_provider_mappings(model_id);
CREATE INDEX idx_mappings_provider_id ON model_provider_mappings(provider_id);
CREATE INDEX idx_mappings_status_model ON model_provider_mappings(status, model_id);

-- Triggers for updated_at
CREATE TRIGGER update_providers_updated_at BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_models_updated_at BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_model_provider_mappings_updated_at BEFORE UPDATE ON model_provider_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments
COMMENT ON TABLE providers IS 'LLM providers (e.g. OpenAI, Anthropic, Google)';
COMMENT ON TABLE models IS 'LLM models (e.g. gpt-4o, claude-sonnet-4-20250514)';
COMMENT ON TABLE model_provider_mappings IS 'Links models to providers with pricing, capabilities, and region';
COMMENT ON COLUMN model_provider_mappings.external_id IS 'The provider''s own identifier for this model';
COMMENT ON COLUMN model_provider_mappings.input_price IS 'Cost per million input tokens';
COMMENT ON COLUMN model_provider_mappings.output_price IS 'Cost per million output tokens';
