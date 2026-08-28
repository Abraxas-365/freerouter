-- ============================================================================
-- Request/Response Content Logging
-- ============================================================================
-- Adds content logging columns to usage_logs:
-- - messages: always stored (the prompt/input messages)
-- - response_body: always stored (LLM response content)
-- - raw_request: debug mode only (full client request body)
-- - raw_response: debug mode only (full upstream response body)
-- - upstream_request: debug mode only (translated request sent to provider)
-- - upstream_response: debug mode only (raw provider response)
-- - is_debug: whether debug mode was enabled for this request

ALTER TABLE usage_logs ADD COLUMN messages JSONB;
ALTER TABLE usage_logs ADD COLUMN response_body JSONB;
ALTER TABLE usage_logs ADD COLUMN raw_request JSONB;
ALTER TABLE usage_logs ADD COLUMN raw_response JSONB;
ALTER TABLE usage_logs ADD COLUMN upstream_request JSONB;
ALTER TABLE usage_logs ADD COLUMN upstream_response JSONB;
ALTER TABLE usage_logs ADD COLUMN is_debug BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN usage_logs.messages IS 'Input messages/prompt (always stored)';
COMMENT ON COLUMN usage_logs.response_body IS 'LLM response content (always stored)';
COMMENT ON COLUMN usage_logs.raw_request IS 'Full client request body (debug mode only)';
COMMENT ON COLUMN usage_logs.raw_response IS 'Full response returned to client (debug mode only)';
COMMENT ON COLUMN usage_logs.upstream_request IS 'Translated request sent to upstream provider (debug mode only)';
COMMENT ON COLUMN usage_logs.upstream_response IS 'Raw response from upstream provider (debug mode only)';
COMMENT ON COLUMN usage_logs.is_debug IS 'Whether debug logging was enabled for this request';

-- ============================================================================
-- Data Retention Configuration
-- ============================================================================

CREATE TABLE data_retention_configs (
    tenant_id VARCHAR(255) PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    retention_days INTEGER NOT NULL DEFAULT 30,
    retain_messages BOOLEAN NOT NULL DEFAULT TRUE,
    retain_response_body BOOLEAN NOT NULL DEFAULT TRUE,
    retain_debug_payloads BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE data_retention_configs IS 'Per-tenant data retention policy for usage log content';
COMMENT ON COLUMN data_retention_configs.retention_days IS 'Days after which content payloads are nullified (0 = never store)';
COMMENT ON COLUMN data_retention_configs.retain_messages IS 'Whether to store input messages at all';
COMMENT ON COLUMN data_retention_configs.retain_response_body IS 'Whether to store LLM response content';
COMMENT ON COLUMN data_retention_configs.retain_debug_payloads IS 'Whether to retain debug payloads beyond retention_days';
