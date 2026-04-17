-- ============================================================
-- Migration 022: Oracle Performance Indexing
-- ============================================================

-- Index for finding nodes by DID quickly during aggregation
CREATE INDEX IF NOT EXISTS idx_nodes_did_device_status ON nodes (did, device_id) WHERE did IS NOT NULL;

-- Composite index for finding unbatched node earnings for a specific date range
-- This is critical for the daily oracle batch process (10M+ records)
CREATE INDEX IF NOT EXISTS idx_node_earnings_aggregation 
ON node_earnings (created_at, device_id) 
WHERE batch_id IS NULL;

-- Index for fast batch status tracking
CREATE INDEX IF NOT EXISTS idx_node_earnings_batch_id ON node_earnings (batch_id) WHERE batch_id IS NOT NULL;

-- Index for oracle batch search
CREATE INDEX IF NOT EXISTS idx_oracle_batches_date_hash ON oracle_batches (batch_date, payload_hash);
