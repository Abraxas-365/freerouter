-- Per-tenant routing strategy configuration
CREATE TABLE routing_configs (
    tenant_id   TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    strategy    TEXT NOT NULL DEFAULT 'cheapest'
        CHECK (strategy IN ('cheapest', 'lowest-latency', 'round-robin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
