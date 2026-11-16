-- Gateway keys FreeRouter issues itself (FREEROUTER_AUTH=none|token).
-- iam mode uses IAMKit service accounts instead and leaves this table empty.
-- Only the SHA-256 of the secret is stored; the secret is shown once.
CREATE TABLE IF NOT EXISTS gateway_keys (
    id          UUID PRIMARY KEY,
    name        TEXT        NOT NULL,
    secret_hash TEXT        NOT NULL UNIQUE,
    permissions TEXT[]      NOT NULL DEFAULT '{}',
    expires_in  TEXT        NOT NULL DEFAULT '',
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at  TIMESTAMPTZ
);
