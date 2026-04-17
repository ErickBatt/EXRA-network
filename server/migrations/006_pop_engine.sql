CREATE TABLE IF NOT EXISTS pop_reward_events (
  id BIGSERIAL PRIMARY KEY,
  device_id TEXT NOT NULL,
  total_emission NUMERIC(20,8) NOT NULL,
  worker_reward NUMERIC(20,8) NOT NULL,
  referral_percent NUMERIC(6,4) NOT NULL DEFAULT 0,
  referral_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
  referrer_device_id TEXT,
  treasury_reward NUMERIC(20,8) NOT NULL,
  reason_code TEXT NOT NULL DEFAULT 'pop_heartbeat',
  policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS treasury_ledger (
  id BIGSERIAL PRIMARY KEY,
  pop_reward_event_id BIGINT REFERENCES pop_reward_events(id),
  amount NUMERIC(20,8) NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS referrer_device_id TEXT;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS referral_count INT DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS total_worker_reward NUMERIC(20,8) DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS total_treasury_reward NUMERIC(20,8) DEFAULT 0;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS total_referral_reward NUMERIC(20,8) DEFAULT 0;
