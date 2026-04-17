-- Migration 018: Pool System (Phase 3)
-- Reputation-based guilds with tiered treasury fees.
--
-- Tier thresholds:
--   Solo     < 10 nodes             treasury_fee = 30%
--   Silver   10–99 nodes            treasury_fee = 20%
--   Gold     100+ nodes, 95% uptime treasury_fee = 15%
--   Platinum 500+ nodes, 98% uptime treasury_fee = 10%
--   Minimum treasury fee always     10%

CREATE TABLE IF NOT EXISTS pools (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL UNIQUE,
    slug            TEXT        NOT NULL UNIQUE,          -- url-safe identifier
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
    -- A node can only be in one pool at a time
    PRIMARY KEY (device_id)
);

CREATE INDEX IF NOT EXISTS idx_pool_members_pool_id ON pool_members (pool_id);
CREATE INDEX IF NOT EXISTS idx_pools_owner ON pools (owner_device_id);
CREATE INDEX IF NOT EXISTS idx_pools_tier  ON pools (tier);

-- Recalculate pool tier whenever node_count or avg_uptime_pct changes.
-- Expressed as a computed column helper (called from Go update logic):
--   solo:     node_count < 10
--   silver:   10 <= node_count < 100
--   gold:     node_count >= 100 AND avg_uptime_pct >= 95
--   platinum: node_count >= 500 AND avg_uptime_pct >= 98

COMMENT ON TABLE pools IS
    'Reputation-based node guilds. Tier determines treasury fee discount.';
COMMENT ON TABLE pool_members IS
    'Node → Pool membership. One active pool per device_id.';

-- FCM push tokens for mobile devices
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

COMMENT ON TABLE push_tokens IS
    'FCM push notification tokens. Used for epoch alerts and payout notifications.';
