-- Fix E3: store worker-reported bytes per session so FinalizeSession can
-- cross-check against gateway-measured bytes. Billing uses MAX of the two,
-- preventing a compromised or misconfigured Gateway from underreporting bytes
-- and leaving workers underpaid.
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS worker_bytes_reported BIGINT NOT NULL DEFAULT 0;
