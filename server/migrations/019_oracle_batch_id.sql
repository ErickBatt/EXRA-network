-- Migration 019: Oracle Batch Tracking for Earnings
-- Adds reference to batching to allow immutable daily accounting.

ALTER TABLE node_earnings ADD COLUMN IF NOT EXISTS batch_id BIGINT DEFAULT NULL;

-- Index for finding unbatched earnings quickly
CREATE INDEX IF NOT EXISTS idx_node_earnings_unbatched ON node_earnings (created_at) WHERE batch_id IS NULL;

-- Track which oracle processed which earnings specifically
ALTER TABLE oracle_batches ADD COLUMN IF NOT EXISTS batch_json JSONB DEFAULT NULL;
COMMENT ON COLUMN oracle_batches.batch_json IS 'Full JSON of {did: amount} for transparency and audit.';
