-- Migration 020: TMA PEAQ Identity Integration
-- Links Telegram users to on-chain DIDs and enables multi-device profile aggregation.

-- 1. Extend tma_users with DID
ALTER TABLE tma_users ADD COLUMN IF NOT EXISTS primary_did TEXT REFERENCES nodes(did) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_tma_users_did ON tma_users(primary_did) WHERE primary_did IS NOT NULL;

-- 2. Add composite index for faster device lookup in TmaAuth
CREATE INDEX IF NOT EXISTS idx_tma_devices_lookup ON tma_devices (telegram_id, device_id);

-- 3. Update existing statuses to be more explicit if needed
COMMENT ON COLUMN tma_users.primary_did IS 'Master DID for the Telegram profile. Used for pooled withdrawals.';
