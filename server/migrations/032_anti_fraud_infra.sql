-- Migration 032: Anti-fraud infrastructure (from root migrations/019_anti_fraud.sql)

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS freeze_reason TEXT DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS frozen_at     TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS updated_at    TIMESTAMPTZ DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_nodes_frozen ON nodes (status) WHERE status = 'frozen';

CREATE TABLE IF NOT EXISTS oracle_probes (
    id            BIGSERIAL   PRIMARY KEY,
    device_id     TEXT        NOT NULL,
    probe_id      TEXT        NOT NULL UNIQUE,
    expected_hash TEXT        NOT NULL,
    result        TEXT        NOT NULL DEFAULT 'pending'
                  CHECK (result IN ('pending','pass','fail')),
    responded_at  TIMESTAMPTZ DEFAULT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oracle_probes_device  ON oracle_probes (device_id);
CREATE INDEX IF NOT EXISTS idx_oracle_probes_pending ON oracle_probes (device_id, result) WHERE result = 'pending';
