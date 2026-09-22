-- Rate limit configs: per-subject RPM + max-concurrent settings.
-- SubjectID "default" is the global fallback applied when no subject-specific config exists.

CREATE TABLE IF NOT EXISTS rate_limit_configs (
    id          UUID        PRIMARY KEY,
    name        TEXT        NOT NULL,
    subject_id  TEXT        NOT NULL,
    rpm         INTEGER     NOT NULL DEFAULT 60,
    max_concurrent INTEGER  NOT NULL DEFAULT 10,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rate_limit_configs_subject_unique UNIQUE (subject_id)
);

CREATE INDEX idx_rate_limit_configs_subject ON rate_limit_configs (subject_id);
