-- Migration 027: payout fulfillment fields
--
-- Until now `payout_requests` had only `pending`, `approved`, `rejected`
-- statuses and no way to record that funds were actually delivered. The
-- ledger was therefore unable to answer the user-facing question
-- "where is my withdrawal?". Operators need a terminal `paid` state plus
-- enough audit fields (tx hash, provider, paid_at, free-form note) to
-- reconcile against the off-ramp service that performed the transfer.
--
-- The Peaq pallet currently has no payout extrinsic (only batch_mint /
-- update_reputations / stake_for_peak), so payouts are fulfilled by an
-- external off-ramp; tx_hash is therefore a generic string, not a
-- Peaq-specific extrinsic hash.

ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS tx_hash         TEXT;
ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS payout_provider TEXT;
ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS paid_at         TIMESTAMPTZ;
ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS payout_note     TEXT;

-- A terminal `paid` row must carry both a tx_hash and a paid_at so we can
-- never end up with half-recorded fulfillments.
ALTER TABLE payout_requests
    DROP CONSTRAINT IF EXISTS payout_requests_paid_consistency_chk;
ALTER TABLE payout_requests
    ADD CONSTRAINT payout_requests_paid_consistency_chk
    CHECK (
        status <> 'paid'
        OR (tx_hash IS NOT NULL AND paid_at IS NOT NULL)
    );

-- Helps the admin payout list and "show me unfulfilled approved payouts"
-- queries; both run frequently from the admin console.
CREATE INDEX IF NOT EXISTS idx_payout_requests_status_updated_at
    ON payout_requests (status, updated_at DESC);
