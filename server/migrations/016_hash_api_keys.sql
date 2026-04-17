-- Migration 016: Hash API keys for secure lookup
-- Stores SHA-256 hashes instead of plaintext keys for authentication lookups.
-- Raw api_key column is preserved for display-back on creation only.

-- Buyers: add hash column, backfill, index
ALTER TABLE buyers ADD COLUMN IF NOT EXISTS api_key_hash TEXT;
UPDATE buyers SET api_key_hash = encode(sha256(api_key::bytea), 'hex') WHERE api_key_hash IS NULL;
ALTER TABLE buyers ALTER COLUMN api_key_hash SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_buyers_api_key_hash ON buyers(api_key_hash);

-- Admin users: add hash column, backfill where key exists, index
ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS api_key_hash TEXT;
UPDATE admin_users SET api_key_hash = encode(sha256(api_key::bytea), 'hex') WHERE api_key IS NOT NULL AND api_key_hash IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_users_api_key_hash ON admin_users(api_key_hash);
