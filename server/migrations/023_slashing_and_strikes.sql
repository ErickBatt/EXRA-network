-- Migration 023: Slashing and Node Strikes
-- Implements Task 7 of Marketplace Architecture v2.1

CREATE TABLE IF NOT EXISTS node_strikes (
    id BIGSERIAL PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for fast strike counting in the last 24h
CREATE INDEX IF NOT EXISTS idx_node_strikes_device_recent ON node_strikes(device_id, created_at);

-- Add stake tracking if not already present
-- (Already added in previous migrations/models but ensuring consistency)
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS stake_exra DECIMAL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS rs_mult DECIMAL DEFAULT 1.0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS feeder_trust_score DECIMAL DEFAULT 0.5;

CREATE TABLE IF NOT EXISTS attestation_logs (
    id BIGSERIAL PRIMARY KEY,
    state_root TEXT NOT NULL,
    node_count INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
