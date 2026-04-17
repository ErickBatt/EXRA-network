-- Migration 017: Gas Pool
-- Tracks TON gas fees collected from user payout requests.
-- Used for Oracle wallet replenishment accounting.

CREATE TABLE IF NOT EXISTS gas_pool (
    id                 BIGSERIAL PRIMARY KEY,
    device_id          TEXT        NOT NULL,
    payout_request_id  TEXT        NOT NULL,   -- FK to payout_requests.id
    fee_ton            NUMERIC(18, 9) NOT NULL CHECK (fee_ton > 0),
    fee_usd            NUMERIC(18, 6) NOT NULL DEFAULT 0,
    collected_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gas_pool_device_id ON gas_pool (device_id);
CREATE INDEX IF NOT EXISTS idx_gas_pool_collected_at ON gas_pool (collected_at DESC);

COMMENT ON TABLE gas_pool IS
    'Append-only ledger of gas fees collected from payout requests. '
    'Used to track how much TON the platform has earned to cover Oracle mint costs.';
