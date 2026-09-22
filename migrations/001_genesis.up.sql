-- 001_genesis.up.sql
-- Foundation tables for FreeRouter v2

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ── Providers ────────────────────────────────────────────────────────
CREATE TABLE providers (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    base_url   TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_providers_slug ON providers (slug);
CREATE INDEX idx_providers_active ON providers (active) WHERE active = true;
