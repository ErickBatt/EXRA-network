-- Migration 021: IP-based Sybil resistance for TMA device linking (G2 from audit)
-- Zero-downtime: additive ALTER TABLE only.
--
-- Strategy: store the /24 (IPv4) or /48 (IPv6) subnet at link-request time.
-- Backend enforces max 3 linked devices per subnet (configurable via env later).
-- Keeps a forensic trail — even rejected links keep their subnet fingerprint.

ALTER TABLE tma_devices ADD COLUMN IF NOT EXISTS linked_ip   TEXT DEFAULT NULL;
ALTER TABLE tma_devices ADD COLUMN IF NOT EXISTS ip_subnet   TEXT DEFAULT NULL;

-- Subnet-level Sybil lookup: count 'linked' devices per subnet efficiently.
CREATE INDEX IF NOT EXISTS idx_tma_devices_subnet_linked
    ON tma_devices (ip_subnet)
    WHERE status = 'linked';

COMMENT ON COLUMN tma_devices.linked_ip IS
    'Raw client IP at link-request time (from X-Real-IP / X-Forwarded-For).';
COMMENT ON COLUMN tma_devices.ip_subnet IS
    'Masked subnet: /24 for IPv4, /48 for IPv6. Used for Sybil detection.';
