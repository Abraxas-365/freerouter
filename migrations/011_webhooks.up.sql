-- Webhook configuration per tenant
CREATE TABLE webhook_configs (
    id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    url           TEXT NOT NULL,
    secret        TEXT NOT NULL,                      -- HMAC signing secret
    events        TEXT[] NOT NULL DEFAULT '{}',        -- e.g. {"request.completed","spending.warning"}
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_configs_tenant ON webhook_configs(tenant_id) WHERE enabled;

-- Webhook delivery log
CREATE TABLE webhook_deliveries (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    webhook_id      TEXT NOT NULL REFERENCES webhook_configs(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',   -- pending, success, failed
    status_code     INTEGER,
    attempts        INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT,
    next_retry_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries(next_retry_at)
    WHERE status = 'pending';
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id, created_at DESC);
