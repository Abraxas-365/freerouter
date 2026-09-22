-- Guardrails: global content-safety configuration, custom rules, and violation log.
-- v2 has no environment/org scoping (IAMKit handles multi-tenancy upstream), so the
-- config table holds a single global row.

CREATE TABLE IF NOT EXISTS guardrail_configs (
    id           UUID        PRIMARY KEY,
    enabled      BOOLEAN     NOT NULL DEFAULT true,
    system_rules JSONB       NOT NULL DEFAULT '{"prompt_injection":{"enabled":true,"action":"block"},"jailbreak":{"enabled":true,"action":"block"},"pii_detection":{"enabled":true,"action":"redact"},"secrets":{"enabled":true,"action":"block"}}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS guardrail_rules (
    id          UUID        PRIMARY KEY,
    name        TEXT        NOT NULL,
    type        TEXT        NOT NULL CHECK (type IN ('blocked_terms', 'custom_regex')),
    config      JSONB       NOT NULL,
    priority    INTEGER     NOT NULL DEFAULT 100,
    enabled     BOOLEAN     NOT NULL DEFAULT true,
    action      TEXT        NOT NULL DEFAULT 'block' CHECK (action IN ('block', 'redact', 'warn')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_guardrail_rules_priority ON guardrail_rules (priority ASC, created_at ASC);

CREATE TABLE IF NOT EXISTS guardrail_violations (
    id              UUID        PRIMARY KEY,
    rule_id         TEXT        NOT NULL,
    rule_name       TEXT        NOT NULL,
    category        TEXT        NOT NULL,
    action_taken    TEXT        NOT NULL,
    matched_pattern TEXT,
    matched_content TEXT,
    model           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_guardrail_violations_created ON guardrail_violations (created_at DESC);
