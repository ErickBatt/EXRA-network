-- Migration 020: Worker marketplace listings
-- Zero-downtime: additive only — no changes to existing tables.
-- Workers create listings from TMA; buyers browse via public API.

CREATE TABLE IF NOT EXISTS worker_listings (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Ownership: linked to TMA-authenticated worker. Cascade delete removes
    -- listings if the worker account is purged (GDPR / Sybil sweep).
    telegram_id    BIGINT        NOT NULL REFERENCES tma_users(telegram_id) ON DELETE CASCADE,
    -- Device: one active listing per device. FK ensures no phantom listings.
    device_id      TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    price_per_gb   NUMERIC(10,4) NOT NULL CHECK (price_per_gb > 0 AND price_per_gb < 1000),
    bandwidth_mbps INT           NOT NULL CHECK (bandwidth_mbps > 0),
    -- Snapshot at listing time — avoids join on every marketplace read.
    gear_score     NUMERIC(5,2)  NOT NULL DEFAULT 0,
    identity_tier  TEXT          NOT NULL DEFAULT 'anon'
                                 CHECK (identity_tier IN ('anon', 'basic', 'peak')),
    -- Sybil guard: minimum PoP sessions completed before listing was allowed.
    -- Stored as snapshot so fraudsters can't delete PoP events post-listing.
    pop_sessions   INT           NOT NULL DEFAULT 0 CHECK (pop_sessions >= 0),
    status         TEXT          NOT NULL DEFAULT 'active'
                                 CHECK (status IN ('active', 'paused', 'deleted')),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    -- One listing per device: worker reprices via UPSERT (ON CONFLICT DO UPDATE).
    CONSTRAINT uq_listing_device UNIQUE (device_id)
);

-- Marketplace browse: buyers filter by status, sort by gear_score/price.
CREATE INDEX IF NOT EXISTS idx_wl_active_score
    ON worker_listings (gear_score DESC, price_per_gb ASC)
    WHERE status = 'active';

-- Worker's own listings (TMA dashboard).
CREATE INDEX IF NOT EXISTS idx_wl_telegram
    ON worker_listings (telegram_id, updated_at DESC);

-- Auto-refresh updated_at on every mutation.
CREATE OR REPLACE FUNCTION _wl_set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_wl_updated ON worker_listings;
CREATE TRIGGER trg_wl_updated
    BEFORE UPDATE ON worker_listings
    FOR EACH ROW EXECUTE FUNCTION _wl_set_updated_at();

COMMENT ON TABLE worker_listings IS
    'Worker-created marketplace listings. Managed via TMA; read publicly by buyers. '
    'Sybil-resistant: creation requires >=3 PoP sessions, ownership enforced by telegram_id.';
