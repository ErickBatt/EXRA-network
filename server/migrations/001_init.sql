CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id TEXT UNIQUE,
    ip TEXT DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    port INTEGER NOT NULL DEFAULT 0,
    country TEXT DEFAULT '',
    device_type TEXT DEFAULT '',
    status TEXT DEFAULT 'offline',
    traffic_bytes BIGINT DEFAULT 0,
    bandwidth_mbps INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT true,
    last_seen TIMESTAMPTZ DEFAULT NOW(),
    last_heartbeat TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS buyers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key TEXT UNIQUE NOT NULL,
    balance_usd NUMERIC(10,4) DEFAULT 0 CHECK (balance_usd >= 0),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id UUID REFERENCES buyers(id),
    node_id UUID REFERENCES nodes(id),
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    bytes_used BIGINT DEFAULT 0,
    cost_usd NUMERIC(10,6) DEFAULT 0,
    active BOOLEAN DEFAULT true,
    billed BOOLEAN DEFAULT false
);

CREATE TABLE IF NOT EXISTS usage_logs (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID REFERENCES sessions(id),
    bytes BIGINT NOT NULL,
    logged_at TIMESTAMPTZ DEFAULT NOW()
);
