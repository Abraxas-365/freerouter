-- Guardrail configs (one per tenant)
CREATE TABLE guardrail_configs (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    system_rules JSONB NOT NULL DEFAULT '{
        "prompt_injection": {"enabled": true, "action": "block"},
        "jailbreak": {"enabled": true, "action": "block"},
        "pii_detection": {"enabled": true, "action": "redact"},
        "secrets": {"enabled": true, "action": "block"},
        "document_leakage": {"enabled": false, "action": "warn"}
    }'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT guardrail_configs_tenant_unique UNIQUE (tenant_id)
);

CREATE INDEX idx_guardrail_configs_tenant ON guardrail_configs(tenant_id);

-- Custom guardrail rules (multiple per tenant)
CREATE TABLE guardrail_rules (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('blocked_terms', 'custom_regex')),
    config      JSONB NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 100,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    action      TEXT NOT NULL DEFAULT 'block' CHECK (action IN ('block', 'redact', 'warn')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_guardrail_rules_tenant ON guardrail_rules(tenant_id);
CREATE INDEX idx_guardrail_rules_priority ON guardrail_rules(priority DESC);

-- Guardrail violation log (audit trail)
CREATE TABLE guardrail_violations (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    rule_id         TEXT NOT NULL,
    rule_name       TEXT NOT NULL,
    category        TEXT NOT NULL,
    action_taken    TEXT NOT NULL CHECK (action_taken IN ('blocked', 'redacted', 'warned')),
    matched_pattern TEXT,
    matched_content TEXT,
    model           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_guardrail_violations_tenant_created ON guardrail_violations(tenant_id, created_at DESC);
CREATE INDEX idx_guardrail_violations_rule_created ON guardrail_violations(rule_id, created_at DESC);
