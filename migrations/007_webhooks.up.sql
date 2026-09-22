-- Webhooks: event subscriptions + delivery attempts with retry tracking.

CREATE TABLE IF NOT EXISTS webhook_configs (
    id          UUID        PRIMARY KEY,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL,
    events      TEXT[]      NOT NULL DEFAULT '{}',
    enabled     BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_configs_enabled ON webhook_configs (enabled) WHERE enabled;

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id            UUID        PRIMARY KEY,
    webhook_id    UUID        NOT NULL REFERENCES webhook_configs(id) ON DELETE CASCADE,
    event_type    TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failed')),
    status_code   INTEGER,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT,
    next_retry_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries (webhook_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_pending ON webhook_deliveries (next_retry_at) WHERE status = 'pending';
