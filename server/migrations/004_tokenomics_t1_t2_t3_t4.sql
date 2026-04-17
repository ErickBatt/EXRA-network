ALTER TABLE nodes ADD COLUMN IF NOT EXISTS device_tier TEXT DEFAULT 'network';
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS is_residential BOOLEAN DEFAULT true;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS asn_org TEXT DEFAULT '';

CREATE TABLE IF NOT EXISTS reward_events (
    id BIGSERIAL PRIMARY KEY,
    device_id TEXT NOT NULL,
    bytes BIGINT NOT NULL DEFAULT 0,
    base_rate_usd_per_gb NUMERIC(10,6) NOT NULL DEFAULT 0,
    tier_multiplier NUMERIC(10,6) NOT NULL DEFAULT 1,
    quality_factor NUMERIC(10,6) NOT NULL DEFAULT 1,
    earned_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
    reason_code TEXT NOT NULL DEFAULT 'ok',
    policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    quarantined BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS oracle_mint_queue (
    id BIGSERIAL PRIMARY KEY,
    reward_event_id BIGINT REFERENCES reward_events(id),
    device_id TEXT NOT NULL,
    amount_EXRA NUMERIC(20,8) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    tx_signature TEXT DEFAULT '',
    error_text TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS burn_events (
    id BIGSERIAL PRIMARY KEY,
    buyer_id UUID,
    input_currency TEXT NOT NULL,
    input_amount NUMERIC(20,8) NOT NULL,
    EXRA_bought NUMERIC(20,8) NOT NULL DEFAULT 0,
    EXRA_burned NUMERIC(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
