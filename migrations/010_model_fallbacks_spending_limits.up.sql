-- Model fallback configuration
CREATE TABLE model_fallbacks (
    id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    model_id      TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    fallback_model_id TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    priority      INTEGER NOT NULL DEFAULT 0,   -- lower = higher priority
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (model_id, fallback_model_id),
    CHECK (model_id != fallback_model_id)
);

CREATE INDEX idx_model_fallbacks_model ON model_fallbacks(model_id, priority) WHERE enabled;

-- Spending limit configuration per tenant
CREATE TABLE spending_limit_configs (
    tenant_id       TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    daily_limit_usd    DOUBLE PRECISION,          -- NULL = no limit
    monthly_limit_usd  DOUBLE PRECISION,          -- NULL = no limit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
