-- 017_tma_device_linking.sql
-- Adds status and request_id to tma_devices to support secure physical verification of link requests.

ALTER TABLE tma_devices ADD COLUMN status VARCHAR(20) DEFAULT 'pending';
ALTER TABLE tma_devices ADD COLUMN request_id UUID;

-- Mark existing devices as linked
UPDATE tma_devices SET status = 'linked' WHERE status IS NULL OR status = 'pending';
