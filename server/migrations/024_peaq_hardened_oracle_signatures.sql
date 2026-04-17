-- Migration 024: PEAQ Hardened Oracle Signatures & On-Chain Finalization
-- Enables tracking of multi-signature consensus batches and their on-chain outcome.

-- 1. Create oracle_signatures table for explicit consensus proof tracking
CREATE TABLE IF NOT EXISTS oracle_signatures (
    id          BIGSERIAL     PRIMARY KEY,
    batch_id    BIGINT        NOT NULL REFERENCES oracle_batches(id) ON DELETE CASCADE,
    oracle_did  TEXT          NOT NULL,
    signature   TEXT          NOT NULL,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Index for fast signature aggregation
CREATE INDEX IF NOT EXISTS idx_oracle_signatures_batch ON oracle_signatures (batch_id);

-- 2. Hardening oracle_batches with on-chain metadata
ALTER TABLE oracle_batches ADD COLUMN IF NOT EXISTS is_finalized_on_chain BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE oracle_batches ADD COLUMN IF NOT EXISTS on_chain_tx_hash    TEXT DEFAULT NULL;

-- 3. Comments for documentation
COMMENT ON TABLE oracle_signatures IS 'Stores the individual sr25519 signatures used in a specific on-chain batch_mint extrinsic.';
COMMENT ON COLUMN oracle_batches.is_finalized_on_chain IS 'TRUE when the batch has been successfully minted on peaq L1.';
COMMENT ON COLUMN oracle_batches.on_chain_tx_hash IS 'The transaction hash of the successful batch_mint extrinsic.';
