-- Usage logs: records every gateway request with token counts, costs, timing.
CREATE TABLE IF NOT EXISTS usage_logs (
    id                UUID PRIMARY KEY,
    key_id            UUID NOT NULL,
    requested_model   TEXT NOT NULL,
    used_model        TEXT NOT NULL,
    provider_id       UUID NOT NULL,
    mapping_id        UUID NOT NULL,

    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens      INTEGER NOT NULL DEFAULT 0,
    cached_tokens     INTEGER NOT NULL DEFAULT 0,

    input_cost        DECIMAL NOT NULL DEFAULT 0,
    output_cost       DECIMAL NOT NULL DEFAULT 0,
    total_cost        DECIMAL NOT NULL DEFAULT 0,

    duration_ms       INTEGER NOT NULL DEFAULT 0,
    streamed          BOOLEAN NOT NULL DEFAULT FALSE,
    status_code       INTEGER NOT NULL DEFAULT 200,
    finish_reason     TEXT NOT NULL DEFAULT '',
    has_error         BOOLEAN NOT NULL DEFAULT FALSE,
    error_message     TEXT NOT NULL DEFAULT '',
    is_fallback       BOOLEAN NOT NULL DEFAULT FALSE,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_logs_created
    ON usage_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_logs_model_created
    ON usage_logs (requested_model, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_created
    ON usage_logs (provider_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_logs_has_error
    ON usage_logs (has_error, created_at DESC)
    WHERE has_error = TRUE;
