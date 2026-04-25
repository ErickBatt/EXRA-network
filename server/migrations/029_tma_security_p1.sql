-- Migration 029: TMA Security P1 hardening
-- Fixes: #3 JWT revocation, #5 device-level request spam, #6 hw fingerprint binding
-- Zero-downtime: additive only (ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS)

-- #3: JWT revocation table.
-- TMAAuth checks this on every request; entries expire naturally so the table stays small.
CREATE TABLE IF NOT EXISTS tma_revoked_sessions (
    jti        TEXT        PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tma_revoked_expires
    ON tma_revoked_sessions (expires_at);

-- #5: updated_at on tma_devices — tracks when a row was last touched so we can
-- rate-limit pending link requests per device_id within a sliding 1-hour window.
ALTER TABLE tma_devices ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- #6: hw_fingerprint on nodes — set at registration time; verified on TMA link approval.
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS hw_fingerprint TEXT DEFAULT NULL;
