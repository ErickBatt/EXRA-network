-- Migration 030: Gas Pool (ported from root migrations/017_gas_pool.sql)
-- Tracks gas fees collected from payout requests.

CREATE TABLE IF NOT EXISTS gas_pool (
    id                 BIGSERIAL PRIMARY KEY,
    device_id          TEXT        NOT NULL,
    payout_request_id  TEXT        NOT NULL,
    fee_ton            NUMERIC(18, 9) NOT NULL CHECK (fee_ton > 0),
    fee_usd            NUMERIC(18, 6) NOT NULL DEFAULT 0,
    collected_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gas_pool_device_id ON gas_pool (device_id);
CREATE INDEX IF NOT EXISTS idx_gas_pool_collected_at ON gas_pool (collected_at DESC);
