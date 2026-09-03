-- Per-tenant rate limit configuration
CREATE TABLE rate_limit_configs (
    tenant_id       TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    rpm             INTEGER NOT NULL DEFAULT 60,
    max_concurrent  INTEGER NOT NULL DEFAULT 10,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
