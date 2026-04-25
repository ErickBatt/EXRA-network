-- Migration 034: IP-based Sybil resistance for TMA linking (from root migrations/021_tma_sybil_ip.sql)

ALTER TABLE tma_devices ADD COLUMN IF NOT EXISTS linked_ip   TEXT DEFAULT NULL;
ALTER TABLE tma_devices ADD COLUMN IF NOT EXISTS ip_subnet   TEXT DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_tma_devices_subnet_linked
    ON tma_devices (ip_subnet)
    WHERE status = 'linked';
