package db

import (
	"database/sql"
	"log"

	"time"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init(dsn string) {
	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	// Dynamic pool management to prevent (EMAXCONNSESSION) errors
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxLifetime(5 * time.Minute)

	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	log.Println("Database connected (pool size: 25)")
	createTables()
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS nodes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			device_id TEXT UNIQUE,
			ip TEXT DEFAULT '',
			address TEXT NOT NULL DEFAULT '',
			port INTEGER NOT NULL DEFAULT 0,
			country TEXT DEFAULT '',
			device_type TEXT DEFAULT '',
			device_tier TEXT DEFAULT 'network',
			is_residential BOOLEAN DEFAULT true,
			asn_org TEXT DEFAULT '',
			status TEXT DEFAULT 'offline',
			traffic_bytes BIGINT DEFAULT 0,
			bandwidth_mbps INTEGER DEFAULT 0,
			active BOOLEAN DEFAULT true,
			last_seen TIMESTAMPTZ DEFAULT NOW(),
			last_heartbeat TIMESTAMPTZ DEFAULT NOW(),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS buyers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			api_key TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE,
			balance_usd NUMERIC(10,4) DEFAULT 0 CHECK (balance_usd >= 0),
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			buyer_id UUID REFERENCES buyers(id),
			node_id UUID REFERENCES nodes(id),
			offer_id UUID,
			started_at TIMESTAMPTZ DEFAULT NOW(),
			ended_at TIMESTAMPTZ,
			bytes_used BIGINT DEFAULT 0,
			cost_usd NUMERIC(10,6) DEFAULT 0,
			locked_price_per_gb NUMERIC(10,4) DEFAULT 1.50,
			active BOOLEAN DEFAULT true,
			billed BOOLEAN DEFAULT false
		)`,
		`CREATE TABLE IF NOT EXISTS offers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			buyer_id UUID REFERENCES buyers(id),
			country TEXT DEFAULT '',
			target_gb NUMERIC(14,6) NOT NULL,
			max_price_per_gb NUMERIC(14,6) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			reserved_exra NUMERIC(20,8) NOT NULL DEFAULT 0,
			settled_exra NUMERIC(20,8) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS usage_logs (
			id BIGSERIAL PRIMARY KEY,
			session_id UUID REFERENCES sessions(id),
			bytes BIGINT NOT NULL,
			logged_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS node_earnings (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL,
			bytes BIGINT NOT NULL DEFAULT 0,
			earned_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS reward_events (
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
		)`,
		`CREATE TABLE IF NOT EXISTS oracle_mint_queue (
			id BIGSERIAL PRIMARY KEY,
			reward_event_id BIGINT REFERENCES reward_events(id),
			device_id TEXT NOT NULL,
			amount_exra NUMERIC(20,8) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			tx_signature TEXT DEFAULT '',
			error_text TEXT DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			next_retry_at TIMESTAMPTZ,
			dlq_reason TEXT DEFAULT '',
			confirmed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS burn_events (
			id BIGSERIAL PRIMARY KEY,
			buyer_id UUID,
			input_currency TEXT NOT NULL,
			input_amount NUMERIC(20,8) NOT NULL,
			exra_bought NUMERIC(20,8) NOT NULL DEFAULT 0,
			exra_burned NUMERIC(20,8) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS payout_requests (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			device_id TEXT NOT NULL,
			recipient_wallet TEXT NOT NULL DEFAULT '',
			amount_usd NUMERIC(10,6) NOT NULL,
			gas_fee_sol NUMERIC(20,9) NOT NULL DEFAULT 0,
			ata_rent_sol NUMERIC(20,9) NOT NULL DEFAULT 0,
			total_fee_sol NUMERIC(20,9) NOT NULL DEFAULT 0,
			net_amount_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
			ata_exists BOOLEAN NOT NULL DEFAULT false,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		// PoP reward events: one row per heartbeat distribution (idempotency_key prevents double-counting).
		`CREATE TABLE IF NOT EXISTS pop_reward_events (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL,
			total_emission NUMERIC(20,8) NOT NULL DEFAULT 0,
			worker_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
			referral_percent NUMERIC(6,4) NOT NULL DEFAULT 0,
			referral_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
			referrer_device_id TEXT DEFAULT NULL,
			treasury_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
			reason_code TEXT NOT NULL DEFAULT 'pop_heartbeat',
			policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
			idempotency_key TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		// treasury_ledger: aggregate treasury inflow per PoP event.
		`CREATE TABLE IF NOT EXISTS treasury_ledger (
			id BIGSERIAL PRIMARY KEY,
			pop_reward_event_id BIGINT REFERENCES pop_reward_events(id),
			amount NUMERIC(20,8) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS compute_tasks (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			buyer_id UUID REFERENCES buyers(id),
			task_type TEXT NOT NULL,
			status TEXT DEFAULT 'pending',
			requirements JSONB DEFAULT '{}'::jsonb,
			min_vram_mb INTEGER DEFAULT 0,
			min_cpu_cores INTEGER DEFAULT 0,
			input_url TEXT,
			output_url TEXT,
			reward_usd NUMERIC(10,4) DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS task_assignments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			task_id UUID REFERENCES compute_tasks(id),
			node_id UUID REFERENCES nodes(id),
			status TEXT DEFAULT 'active',
			result_hash TEXT,
			started_at TIMESTAMPTZ DEFAULT NOW(),
			ended_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS swap_events (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL,
			exra_amount NUMERIC(20,8) NOT NULL,
			usdc_amount NUMERIC(20,8) NOT NULL,
			spread_usd NUMERIC(20,8) NOT NULL,
			status TEXT DEFAULT 'completed',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS treasury_vault (
			id SERIAL PRIMARY KEY,
			usdc_balance NUMERIC(20,8) DEFAULT 1000.0,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS admin_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS admin_audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor_id UUID,
			actor_email TEXT NOT NULL,
			role TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			request_id TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			payload_redacted JSONB NOT NULL DEFAULT '{}'::jsonb,
			result TEXT NOT NULL DEFAULT 'success',
			error_text TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
	}
	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Printf("Table creation warning: %v", err)
		}
	}

	// Keep existing databases compatible with new P0 billing flow.
	compatQueries := []string{
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS billed BOOLEAN DEFAULT false`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS device_id TEXT UNIQUE`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS ip TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS device_type TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS device_tier TEXT DEFAULT 'network'`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS is_residential BOOLEAN DEFAULT true`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS asn_org TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'offline'`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS traffic_bytes BIGINT DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS last_seen TIMESTAMPTZ DEFAULT NOW()`,
		// PoP referral + 3-stream aggregate columns (additive, safe defaults).
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS referrer_device_id TEXT DEFAULT NULL`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS referral_count INTEGER DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS total_worker_reward NUMERIC(20,8) DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS total_referral_reward NUMERIC(20,8) DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS total_treasury_reward NUMERIC(20,8) DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS cpu_model TEXT DEFAULT ''`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS cpu_cores INTEGER DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS vram_mb INTEGER DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS ram_mb INTEGER DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS price_per_gb NUMERIC(10,4) DEFAULT 1.50`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS auto_price BOOLEAN DEFAULT true`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS locked_price_per_gb NUMERIC(10,4) DEFAULT 1.50`,
		`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS offer_id UUID`,
		`CREATE TABLE IF NOT EXISTS offers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			buyer_id UUID REFERENCES buyers(id),
			country TEXT DEFAULT '',
			target_gb NUMERIC(14,6) NOT NULL,
			max_price_per_gb NUMERIC(14,6) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			reserved_exra NUMERIC(20,8) NOT NULL DEFAULT 0,
			settled_exra NUMERIC(20,8) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE OR REPLACE VIEW market_avg_price AS
		 SELECT country, AVG(price_per_gb) as avg_price, COUNT(*) as node_count
		 FROM nodes WHERE status='online'
		 GROUP BY country`,
		// Compute tasks requirements
		`ALTER TABLE compute_tasks ADD COLUMN IF NOT EXISTS min_vram_mb INTEGER DEFAULT 0`,
		`ALTER TABLE compute_tasks ADD COLUMN IF NOT EXISTS min_cpu_cores INTEGER DEFAULT 0`,
		`ALTER TABLE oracle_mint_queue DROP CONSTRAINT IF EXISTS oracle_mint_queue_reward_event_id_fkey`,
		// PoP tables (idempotent — already created above, kept for compat on existing DBs).
		`CREATE TABLE IF NOT EXISTS pop_reward_events (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL,
			total_emission NUMERIC(20,8) NOT NULL DEFAULT 0,
			worker_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
			referral_percent NUMERIC(6,4) NOT NULL DEFAULT 0,
			referral_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
			referrer_device_id TEXT DEFAULT NULL,
			treasury_reward NUMERIC(20,8) NOT NULL DEFAULT 0,
			reason_code TEXT NOT NULL DEFAULT 'pop_heartbeat',
			policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
			idempotency_key TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS treasury_ledger (
			id BIGSERIAL PRIMARY KEY,
			pop_reward_event_id BIGINT REFERENCES pop_reward_events(id),
			amount NUMERIC(20,8) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS node_earnings (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL,
			bytes BIGINT NOT NULL DEFAULT 0,
			earned_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS payout_requests (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			device_id TEXT NOT NULL,
			recipient_wallet TEXT NOT NULL DEFAULT '',
			amount_usd NUMERIC(10,6) NOT NULL,
			gas_fee_sol NUMERIC(20,9) NOT NULL DEFAULT 0,
			ata_rent_sol NUMERIC(20,9) NOT NULL DEFAULT 0,
			total_fee_sol NUMERIC(20,9) NOT NULL DEFAULT 0,
			net_amount_usd NUMERIC(10,6) NOT NULL DEFAULT 0,
			ata_exists BOOLEAN NOT NULL DEFAULT false,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS recipient_wallet TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS gas_fee_sol NUMERIC(20,9) NOT NULL DEFAULT 0`,
		`ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS ata_rent_sol NUMERIC(20,9) NOT NULL DEFAULT 0`,
		`ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS total_fee_sol NUMERIC(20,9) NOT NULL DEFAULT 0`,
		`ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS net_amount_usd NUMERIC(10,6) NOT NULL DEFAULT 0`,
		`ALTER TABLE payout_requests ADD COLUMN IF NOT EXISTS ata_exists BOOLEAN NOT NULL DEFAULT false`,
		`CREATE TABLE IF NOT EXISTS reward_events (
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
		)`,
		`CREATE TABLE IF NOT EXISTS oracle_mint_queue (
			id BIGSERIAL PRIMARY KEY,
			reward_event_id BIGINT REFERENCES reward_events(id),
			device_id TEXT NOT NULL,
			amount_exra NUMERIC(20,8) NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			tx_signature TEXT DEFAULT '',
			error_text TEXT DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			next_retry_at TIMESTAMPTZ,
			dlq_reason TEXT DEFAULT '',
			confirmed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`ALTER TABLE oracle_mint_queue ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE oracle_mint_queue ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ`,
		`ALTER TABLE oracle_mint_queue ADD COLUMN IF NOT EXISTS dlq_reason TEXT DEFAULT ''`,
		`ALTER TABLE oracle_mint_queue ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ`,
		`CREATE TABLE IF NOT EXISTS burn_events (
			id BIGSERIAL PRIMARY KEY,
			buyer_id UUID,
			input_currency TEXT NOT NULL,
			input_amount NUMERIC(20,8) NOT NULL,
			exra_bought NUMERIC(20,8) NOT NULL DEFAULT 0,
			exra_burned NUMERIC(20,8) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`ALTER TABLE oracle_mint_queue DROP COLUMN IF EXISTS amount_kilo`,
		`ALTER TABLE oracle_mint_queue DROP COLUMN IF EXISTS "amount_EXRA"`,
		`ALTER TABLE oracle_mint_queue DROP COLUMN IF EXISTS amount`,
		`ALTER TABLE oracle_mint_queue ADD COLUMN IF NOT EXISTS amount_exra NUMERIC(20,8) NOT NULL DEFAULT 0`,
		`DELETE FROM task_assignments; UPDATE compute_tasks SET status = 'failed' WHERE status = 'pending' OR status = 'assigned'`,
		`DO $$ BEGIN IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='burn_events' AND column_name='EXRA_bought') THEN ALTER TABLE burn_events RENAME COLUMN EXRA_bought TO exra_bought; END IF; END $$`,
		`DO $$ BEGIN IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='burn_events' AND column_name='EXRA_burned') THEN ALTER TABLE burn_events RENAME COLUMN EXRA_burned TO exra_burned; END IF; END $$`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'buyers_balance_non_negative'
			) THEN
				ALTER TABLE buyers
				ADD CONSTRAINT buyers_balance_non_negative CHECK (balance_usd >= 0);
			END IF;
		END $$`,
		`ALTER TABLE buyers ADD COLUMN IF NOT EXISTS email TEXT UNIQUE`,
		// Security: hash-based API key lookup (migration 016)
		`ALTER TABLE buyers ADD COLUMN IF NOT EXISTS api_key_hash TEXT`,
		`UPDATE buyers SET api_key_hash = encode(sha256(api_key::bytea), 'hex') WHERE api_key_hash IS NULL AND api_key IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_buyers_api_key_hash ON buyers(api_key_hash)`,
		`ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS api_key TEXT`,
		`ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS api_key_hash TEXT`,
		`UPDATE admin_users SET api_key_hash = encode(sha256(api_key::bytea), 'hex') WHERE api_key IS NOT NULL AND api_key_hash IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_users_api_key_hash ON admin_users(api_key_hash)`,
		// Gas pool (migration 017)
		`CREATE TABLE IF NOT EXISTS gas_pool (
			id                 BIGSERIAL PRIMARY KEY,
			device_id          TEXT NOT NULL,
			payout_request_id  TEXT NOT NULL,
			fee_chain          NUMERIC(18,9) NOT NULL,
			fee_usd            NUMERIC(18,6) NOT NULL DEFAULT 0,
			collected_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		// Pool system (migration 018)
		`CREATE TABLE IF NOT EXISTS pools (
			id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name             TEXT NOT NULL UNIQUE,
			slug             TEXT NOT NULL UNIQUE,
			owner_device_id  TEXT NOT NULL,
			description      TEXT NOT NULL DEFAULT '',
			node_count       INT NOT NULL DEFAULT 0,
			avg_uptime_pct   NUMERIC(5,2) NOT NULL DEFAULT 0,
			tier             TEXT NOT NULL DEFAULT 'solo',
			treasury_fee_pct NUMERIC(5,2) NOT NULL DEFAULT 30,
			total_earned_exra NUMERIC(24,9) NOT NULL DEFAULT 0,
			is_public        BOOLEAN NOT NULL DEFAULT true,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS pool_members (
			pool_id    UUID NOT NULL REFERENCES pools(id) ON DELETE CASCADE,
			device_id  TEXT NOT NULL,
			joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (device_id)
		)`,
		// Push tokens (FCM)
		`CREATE TABLE IF NOT EXISTS push_tokens (
			id         BIGSERIAL PRIMARY KEY,
			device_id  TEXT NOT NULL,
			fcm_token  TEXT NOT NULL,
			platform   TEXT NOT NULL DEFAULT 'android',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (device_id, fcm_token)
		)`,
		// nodes uptime_pct + freeze columns (migrations 018-019)
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS uptime_pct    NUMERIC(5,2) DEFAULT 0`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS freeze_reason TEXT DEFAULT NULL`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS frozen_at     TIMESTAMPTZ DEFAULT NULL`,
		`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS updated_at    TIMESTAMPTZ DEFAULT NOW()`,
		// Oracle probes table (migration 019)
		`CREATE TABLE IF NOT EXISTS oracle_probes (
			id            BIGSERIAL   PRIMARY KEY,
			device_id     TEXT        NOT NULL,
			probe_id      TEXT        NOT NULL UNIQUE,
			expected_hash TEXT        NOT NULL,
			result        TEXT        NOT NULL DEFAULT 'pending',
			responded_at  TIMESTAMPTZ DEFAULT NULL,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS swap_events (
			id BIGSERIAL PRIMARY KEY,
			device_id TEXT NOT NULL,
			exra_amount NUMERIC(20,8) NOT NULL,
			usdc_amount NUMERIC(20,8) NOT NULL,
			spread_usd NUMERIC(20,8) NOT NULL,
			status TEXT DEFAULT 'completed',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS treasury_vault (
			id SERIAL PRIMARY KEY,
			usdc_balance NUMERIC(20,8) DEFAULT 1000.0,
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`INSERT INTO treasury_vault (usdc_balance) SELECT 1000.0 WHERE NOT EXISTS (SELECT 1 FROM treasury_vault)`,
		`CREATE TABLE IF NOT EXISTS admin_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email TEXT UNIQUE NOT NULL,
			role TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS admin_audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor_id UUID,
			actor_email TEXT NOT NULL,
			role TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			request_id TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			payload_redacted JSONB NOT NULL DEFAULT '{}'::jsonb,
			result TEXT NOT NULL DEFAULT 'success',
			error_text TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tma_users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			telegram_id BIGINT UNIQUE NOT NULL,
			telegram_username TEXT NOT NULL DEFAULT '',
			telegram_first_name TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			last_seen_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tma_devices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			telegram_id BIGINT NOT NULL REFERENCES tma_users(telegram_id) ON DELETE CASCADE,
			device_id TEXT NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
			linked_at TIMESTAMPTZ DEFAULT NOW(),
			UNIQUE(telegram_id, device_id)
		)`,
	}
	for _, q := range compatQueries {
		if _, err := DB.Exec(q); err != nil {
			log.Printf("Schema compatibility warning: %v", err)
		}
	}
	log.Println("Database schema ready")
}
