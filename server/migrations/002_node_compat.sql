ALTER TABLE sessions ADD COLUMN IF NOT EXISTS billed BOOLEAN DEFAULT false;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS device_id TEXT UNIQUE;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS ip TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS device_type TEXT DEFAULT '';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'offline';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS traffic_bytes BIGINT DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'buyers_balance_non_negative'
    ) THEN
        ALTER TABLE buyers
        ADD CONSTRAINT buyers_balance_non_negative CHECK (balance_usd >= 0);
    END IF;
END $$;
