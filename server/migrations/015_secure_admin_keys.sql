-- Migration 015_secure_admin_keys.sql
ALTER TABLE admin_users ADD COLUMN api_key TEXT UNIQUE;
CREATE INDEX idx_admin_users_api_key ON admin_users(api_key);

-- For existing admins, we should generate an initial key if they don't have one, 
-- but for now we leave it NULL and require manual assignment or a script.
