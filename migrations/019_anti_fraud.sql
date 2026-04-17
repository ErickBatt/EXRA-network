-- Migration 019: Anti-fraud infrastructure
-- Adds freeze support to nodes, oracle_probes table for traffic verification.

-- Node freeze columns
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS freeze_reason TEXT DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS frozen_at     TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS updated_at    TIMESTAMPTZ DEFAULT NOW();

-- Constraint: frozen nodes cannot be online
-- (enforced at app level; DB level is advisory)
CREATE INDEX IF NOT EXISTS idx_nodes_frozen ON nodes (status) WHERE status = 'frozen';

-- Oracle probes: control traffic sent by oracle to detect fake traffic
CREATE TABLE IF NOT EXISTS oracle_probes (
    id            BIGSERIAL   PRIMARY KEY,
    device_id     TEXT        NOT NULL,
    probe_id      TEXT        NOT NULL UNIQUE,     -- UUID sent with probe request
    expected_hash TEXT        NOT NULL,            -- SHA-256 of probe payload
    result        TEXT        NOT NULL DEFAULT 'pending'
                  CHECK (result IN ('pending','pass','fail')),
    responded_at  TIMESTAMPTZ DEFAULT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oracle_probes_device ON oracle_probes (device_id);
CREATE INDEX IF NOT EXISTS idx_oracle_probes_pending ON oracle_probes (device_id, result) WHERE result = 'pending';

COMMENT ON TABLE oracle_probes IS
    'Oracle sends a known payload to nodes; hash mismatch triggers FreezeNode().';
