-- Routing configs: per-subject routing strategy settings.
-- SubjectID "default" is the global fallback applied when no subject-specific config exists.

CREATE TABLE IF NOT EXISTS routing_configs (
    id          UUID        PRIMARY KEY,
    subject_id  TEXT        NOT NULL,
    strategy    TEXT        NOT NULL DEFAULT 'cheapest',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT routing_configs_subject_unique UNIQUE (subject_id),
    CONSTRAINT routing_configs_strategy_valid CHECK (strategy IN ('cheapest', 'lowest-latency', 'round-robin'))
);

CREATE INDEX idx_routing_configs_subject ON routing_configs (subject_id);
