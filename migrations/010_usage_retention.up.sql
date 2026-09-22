-- Usage log data retention: global config (days + field toggles) and background
-- purge worker support. v2 has no environment/org scoping (IAMKit handles
-- multi-tenancy upstream), so this is a single global row, same pattern as
-- guardrail_configs.

CREATE TABLE IF NOT EXISTS usage_retention_config (
    id                    UUID        PRIMARY KEY,
    retention_days        INTEGER     NOT NULL DEFAULT 90,
    retain_messages       BOOLEAN     NOT NULL DEFAULT true,
    retain_response_body  BOOLEAN     NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
