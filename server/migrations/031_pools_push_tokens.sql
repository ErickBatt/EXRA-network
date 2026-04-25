-- Migration 031: Pool System + FCM Push Tokens (from root migrations/018_pools.sql)

CREATE TABLE IF NOT EXISTS pools (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL UNIQUE,
    slug            TEXT        NOT NULL UNIQUE,
    owner_device_id TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    node_count      INT         NOT NULL DEFAULT 0,
    avg_uptime_pct  NUMERIC(5,2) NOT NULL DEFAULT 0,
    tier            TEXT        NOT NULL DEFAULT 'solo'
                    CHECK (tier IN ('solo','silver','gold','platinum')),
    treasury_fee_pct NUMERIC(5,2) NOT NULL DEFAULT 30
                    CHECK (treasury_fee_pct >= 10 AND treasury_fee_pct <= 30),
    total_earned_exra NUMERIC(24,9) NOT NULL DEFAULT 0,
    is_public       BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pool_members (
    pool_id     UUID        NOT NULL REFERENCES pools(id) ON DELETE CASCADE,
    device_id   TEXT        NOT NULL,
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (device_id)
);

CREATE INDEX IF NOT EXISTS idx_pool_members_pool_id ON pool_members (pool_id);
CREATE INDEX IF NOT EXISTS idx_pools_owner ON pools (owner_device_id);
CREATE INDEX IF NOT EXISTS idx_pools_tier  ON pools (tier);

CREATE TABLE IF NOT EXISTS push_tokens (
    id          BIGSERIAL   PRIMARY KEY,
    device_id   TEXT        NOT NULL,
    fcm_token   TEXT        NOT NULL,
    platform    TEXT        NOT NULL DEFAULT 'android' CHECK (platform IN ('android','ios')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (device_id, fcm_token)
);

CREATE INDEX IF NOT EXISTS idx_push_tokens_device ON push_tokens (device_id);
