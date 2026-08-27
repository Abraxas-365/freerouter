-- ============================================================================
-- Usage Logs (request logging + token tracking)
-- ============================================================================

CREATE TABLE usage_logs (
    id VARCHAR(255) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    api_key_id VARCHAR(255),
    provider_key_id VARCHAR(255),

    -- Request info
    requested_model VARCHAR(255) NOT NULL,  -- Model the client requested
    used_model VARCHAR(255) NOT NULL,       -- External model sent upstream
    used_provider VARCHAR(255) NOT NULL,    -- Provider that served the request
    mapping_id VARCHAR(255) NOT NULL,       -- model_provider_mapping ID

    -- Token counts
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,

    -- Cost (USD)
    input_cost DECIMAL NOT NULL DEFAULT 0,
    output_cost DECIMAL NOT NULL DEFAULT 0,
    total_cost DECIMAL NOT NULL DEFAULT 0,

    -- Request metadata
    duration_ms INTEGER NOT NULL DEFAULT 0,
    streamed BOOLEAN NOT NULL DEFAULT FALSE,
    status_code INTEGER NOT NULL DEFAULT 200,
    finish_reason VARCHAR(100) NOT NULL DEFAULT '',
    has_error BOOLEAN NOT NULL DEFAULT FALSE,
    error_message TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_usage_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- Query patterns: by tenant + time range, by model, by provider
CREATE INDEX idx_usage_logs_tenant_created ON usage_logs(tenant_id, created_at DESC);
CREATE INDEX idx_usage_logs_tenant_model ON usage_logs(tenant_id, requested_model);
CREATE INDEX idx_usage_logs_tenant_provider ON usage_logs(tenant_id, used_provider);
CREATE INDEX idx_usage_logs_created ON usage_logs(created_at DESC);
CREATE INDEX idx_usage_logs_provider_key ON usage_logs(provider_key_id);

COMMENT ON TABLE usage_logs IS 'Per-request usage log with token counts, costs, and metadata';
COMMENT ON COLUMN usage_logs.requested_model IS 'Model ID the client sent in the request';
COMMENT ON COLUMN usage_logs.used_model IS 'External model identifier sent to the upstream provider';
COMMENT ON COLUMN usage_logs.input_cost IS 'Cost for prompt tokens in USD';
COMMENT ON COLUMN usage_logs.output_cost IS 'Cost for completion tokens in USD';
COMMENT ON COLUMN usage_logs.total_cost IS 'Total cost (input + output) in USD';
