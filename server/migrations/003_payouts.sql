CREATE TABLE IF NOT EXISTS node_earnings (
    id BIGSERIAL PRIMARY KEY,
    device_id TEXT NOT NULL,
    bytes BIGINT NOT NULL DEFAULT 0,
    earned_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payout_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id TEXT NOT NULL,
    amount_usd NUMERIC(10,6) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
