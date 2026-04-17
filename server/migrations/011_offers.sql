CREATE TABLE IF NOT EXISTS offers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  buyer_id UUID REFERENCES buyers(id),
  country TEXT DEFAULT '',
  target_gb NUMERIC(14,6) NOT NULL,
  max_price_per_gb NUMERIC(14,6) NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  reserved_exra NUMERIC(20,8) NOT NULL DEFAULT 0,
  settled_exra NUMERIC(20,8) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS offer_id UUID;
