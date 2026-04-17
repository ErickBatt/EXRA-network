-- Migration 026: Cleanup obsolete oracle_sig column
-- This was replaced by the oracle_signatures table for multi-oracle consensus.

ALTER TABLE oracle_batches DROP COLUMN IF EXISTS oracle_sig;
