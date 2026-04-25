-- Migration 033: Worker marketplace listings (from root migrations/020_worker_listings.sql)
-- THIS IS THE TABLE BEHIND /api/tma/lots — missing from prod caused 500 errors.

CREATE TABLE IF NOT EXISTS worker_listings (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id    BIGINT        NOT NULL REFERENCES tma_users(telegram_id) ON DELETE CASCADE,
    device_id      TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    price_per_gb   NUMERIC(10,4) NOT NULL CHECK (price_per_gb > 0 AND price_per_gb < 1000),
    bandwidth_mbps INT           NOT NULL CHECK (bandwidth_mbps > 0),
    gear_score     NUMERIC(5,2)  NOT NULL DEFAULT 0,
    identity_tier  TEXT          NOT NULL DEFAULT 'anon'
                                 CHECK (identity_tier IN ('anon', 'basic', 'peak')),
    pop_sessions   INT           NOT NULL DEFAULT 0 CHECK (pop_sessions >= 0),
    status         TEXT          NOT NULL DEFAULT 'active'
                                 CHECK (status IN ('active', 'paused', 'deleted')),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_listing_device UNIQUE (device_id)
);

CREATE INDEX IF NOT EXISTS idx_wl_active_score
    ON worker_listings (gear_score DESC, price_per_gb ASC)
    WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_wl_telegram
    ON worker_listings (telegram_id, updated_at DESC);

CREATE OR REPLACE FUNCTION _wl_set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_wl_updated ON worker_listings;
CREATE TRIGGER trg_wl_updated
    BEFORE UPDATE ON worker_listings
    FOR EACH ROW EXECUTE FUNCTION _wl_set_updated_at();
