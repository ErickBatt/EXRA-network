-- ============================================================
-- Migration 018: PEAQ DePIN — DID, Identity Tiers & Anti-Fraud Infrastructure
-- ============================================================
-- Revision 2: Fixed FLOAT→NUMERIC for financial columns, fixed UNIQUE constraint
-- on feeder_assignments, added FK references to nodes table.
-- ============================================================

-- ── 1. Extend nodes table with DID & Identity Tier columns ──────────────────

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS did                  TEXT         UNIQUE;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS identity_tier        TEXT         NOT NULL DEFAULT 'anon'
    CHECK (identity_tier IN ('anon', 'peak'));
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS stake_exra           NUMERIC(18,9) NOT NULL DEFAULT 0
    CHECK (stake_exra >= 0);
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS timelock_hours       INT           NOT NULL DEFAULT 24
    CHECK (timelock_hours >= 0 AND timelock_hours <= 24);
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS timelock_decay_days  INT           NOT NULL DEFAULT 0
    CHECK (timelock_decay_days >= 0);
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS feeder_trust_score   NUMERIC(4,3)  NOT NULL DEFAULT 0.500
    CHECK (feeder_trust_score >= 0 AND feeder_trust_score <= 1.000);
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS rs_mult              NUMERIC(4,3)  NOT NULL DEFAULT 0.500
    CHECK (rs_mult >= 0 AND rs_mult <= 2.000);
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS did_verified_at      TIMESTAMPTZ   DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS honest_days_streak   INT           NOT NULL DEFAULT 0
    CHECK (honest_days_streak >= 0);
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS last_rs_update       TIMESTAMPTZ   DEFAULT NOW();

-- Index: fast DID lookups (authentication and oracle flow)
CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_did ON nodes (did) WHERE did IS NOT NULL;

-- Index: tier-based marketplace filtering
CREATE INDEX IF NOT EXISTS idx_nodes_identity_tier ON nodes (identity_tier);

-- Index: Feeder assignment eligibility — compound for fast query
-- GS>300 check happens at application layer (rs_mult proxy: >0.6 ≈ GS>300)
CREATE INDEX IF NOT EXISTS idx_nodes_feeder_eligible
    ON nodes (identity_tier, stake_exra, rs_mult, ip)
    WHERE active = true AND status != 'frozen';

-- ── 2. Feeder assignments table ─────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS feeder_assignments (
    id               BIGSERIAL     PRIMARY KEY,
    target_device_id TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    feeder_device_id TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    target_subnet    TEXT          NOT NULL,    -- /24 subnet of target (collision detection)
    feeder_subnet    TEXT          NOT NULL,    -- /24 subnet of feeder (must differ from target)
    assigned_date    DATE          NOT NULL DEFAULT CURRENT_DATE,  -- explicit date column
    assigned_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW() + INTERVAL '1 hour',
    status           TEXT          NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'reported', 'expired', 'evaluated')),
    -- One feeder can only be assigned to one target per day (prevents spam)
    CONSTRAINT uq_feeder_assignment_per_day UNIQUE (target_device_id, feeder_device_id, assigned_date)
);

CREATE INDEX IF NOT EXISTS idx_feeder_assignments_target  ON feeder_assignments (target_device_id, status);
CREATE INDEX IF NOT EXISTS idx_feeder_assignments_feeder  ON feeder_assignments (feeder_device_id, status);
CREATE INDEX IF NOT EXISTS idx_feeder_assignments_expires ON feeder_assignments (expires_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_feeder_assignments_date    ON feeder_assignments (assigned_date DESC);

-- ── 3. Feeder reports table ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS feeder_reports (
    id               BIGSERIAL     PRIMARY KEY,
    assignment_id    BIGINT        NOT NULL REFERENCES feeder_assignments(id) ON DELETE CASCADE,
    feeder_device_id TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    target_device_id TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    verdict          TEXT          NOT NULL CHECK (verdict IN ('honest', 'fraud')),
    reported_bytes   BIGINT        NOT NULL DEFAULT 0 CHECK (reported_bytes >= 0),
    expected_bytes   BIGINT        NOT NULL DEFAULT 0 CHECK (expected_bytes >= 0),
    confidence       NUMERIC(4,3)  NOT NULL DEFAULT 1.000
        CHECK (confidence > 0 AND confidence <= 1.000),
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    evaluated_at     TIMESTAMPTZ   DEFAULT NULL,
    evaluation_result TEXT         DEFAULT NULL
        CHECK (evaluation_result IN ('correct', 'false_positive', 'false_negative') OR evaluation_result IS NULL)
);

CREATE INDEX IF NOT EXISTS idx_feeder_reports_target     ON feeder_reports (target_device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_feeder_reports_feeder     ON feeder_reports (feeder_device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_feeder_reports_assignment ON feeder_reports (assignment_id);
-- For consensus calculation: count verdicts by target quickly
CREATE INDEX IF NOT EXISTS idx_feeder_reports_verdict_by_target
    ON feeder_reports (target_device_id, verdict, created_at DESC);

-- ── 4. Canary tasks table ───────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS canary_tasks (
    id               BIGSERIAL     PRIMARY KEY,
    device_id        TEXT          NOT NULL REFERENCES nodes(device_id) ON DELETE CASCADE,
    task_type        TEXT          NOT NULL DEFAULT 'proxy_hash'
        CHECK (task_type IN ('proxy_hash', 'bandwidth_probe', 'latency_check')),
    expected_result  TEXT          NOT NULL,
    submitted_result TEXT          DEFAULT NULL,
    injected_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    responded_at     TIMESTAMPTZ   DEFAULT NULL,
    result           TEXT          NOT NULL DEFAULT 'pending'
        CHECK (result IN ('pending', 'pass', 'fail', 'timeout')),
    penalty_applied  BOOLEAN       NOT NULL DEFAULT FALSE,
    -- Prevent multiple pending canaries for same device simultaneously
    CONSTRAINT uq_canary_one_pending_per_device
        EXCLUDE USING btree (device_id WITH =) WHERE (result = 'pending')
);

CREATE INDEX IF NOT EXISTS idx_canary_tasks_device  ON canary_tasks (device_id, result);
CREATE INDEX IF NOT EXISTS idx_canary_tasks_pending ON canary_tasks (device_id, injected_at DESC) WHERE result = 'pending';
CREATE INDEX IF NOT EXISTS idx_canary_tasks_timeout ON canary_tasks (injected_at) WHERE result = 'pending';

-- ── 5. DID payout velocity table ────────────────────────────────────────────
-- Tracks ONE payout per DID per 24h. eligible_at is computed at app layer
-- (not hardcoded default) to account for Decay Boost.

CREATE TABLE IF NOT EXISTS did_payout_velocity (
    id                       BIGSERIAL     PRIMARY KEY,
    did                      TEXT          NOT NULL,
    payout_id                TEXT          NOT NULL UNIQUE,  -- FK would require UUID; keep TEXT for flexibility
    amount_before_tax        NUMERIC(18,9) NOT NULL CHECK (amount_before_tax >= 0),
    tax_amount               NUMERIC(18,9) NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    net_amount               NUMERIC(18,9) NOT NULL CHECK (net_amount >= 0),
    tier_at_payout           TEXT          NOT NULL DEFAULT 'anon'
        CHECK (tier_at_payout IN ('anon', 'peak')),
    timelock_hours_at_payout INT           NOT NULL DEFAULT 24 CHECK (timelock_hours_at_payout >= 0),
    created_at               TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    eligible_at              TIMESTAMPTZ   NOT NULL  -- set by application: NOW() + timelock_hours_remaining
);

CREATE INDEX IF NOT EXISTS idx_did_velocity_did         ON did_payout_velocity (did, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_did_velocity_eligible_at ON did_payout_velocity (did, eligible_at);
-- Constraint: only one active payout per DID (eligible_at in future = locked)
CREATE INDEX IF NOT EXISTS idx_did_velocity_locked ON did_payout_velocity (did, eligible_at)
    WHERE eligible_at > NOW();

-- ── 6. Sybil events log ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS sybil_events (
    id           BIGSERIAL     PRIMARY KEY,
    ip           TEXT          NOT NULL,
    subnet_24    TEXT          NOT NULL,
    did_count    INT           NOT NULL CHECK (did_count > 0),
    blocked_did  TEXT          DEFAULT NULL,
    device_id    TEXT          DEFAULT NULL,
    action_taken TEXT          NOT NULL DEFAULT 'blocked'
        CHECK (action_taken IN ('blocked', 'flagged', 'freeze_initiated')),
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sybil_events_ip         ON sybil_events (ip, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sybil_events_subnet     ON sybil_events (subnet_24, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sybil_events_created_at ON sybil_events (created_at DESC);

-- ── 7. Oracle batches table ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS oracle_batches (
    id            BIGSERIAL     PRIMARY KEY,
    batch_date    DATE          NOT NULL,
    oracle_id     TEXT          NOT NULL,
    oracle_sig    TEXT          NOT NULL,
    did_count     INT           NOT NULL DEFAULT 0 CHECK (did_count >= 0),
    total_credits NUMERIC(18,9) NOT NULL DEFAULT 0 CHECK (total_credits >= 0),
    fraud_dids    TEXT[]        NOT NULL DEFAULT '{}',
    payload_hash  TEXT          NOT NULL,
    status        TEXT          NOT NULL DEFAULT 'received'
        CHECK (status IN ('received', 'consensus', 'disputed', 'frozen', 'applied')),
    applied_at    TIMESTAMPTZ   DEFAULT NULL,  -- when batch_mint was executed
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_oracle_batch_per_day UNIQUE (batch_date, oracle_id)
);

CREATE INDEX IF NOT EXISTS idx_oracle_batches_date    ON oracle_batches (batch_date DESC);
CREATE INDEX IF NOT EXISTS idx_oracle_batches_status  ON oracle_batches (status, batch_date DESC);
-- For consensus check: count batches per date quickly
CREATE INDEX IF NOT EXISTS idx_oracle_batches_consensus
    ON oracle_batches (batch_date, payload_hash, status);

-- ── 8. Comments ──────────────────────────────────────────────────────────────

COMMENT ON COLUMN nodes.did              IS 'peaq DID — unique on-chain identity. NULL for legacy nodes.';
COMMENT ON COLUMN nodes.identity_tier    IS 'anon: 25% tax, 24h timelock. peak: 0% tax, instant.';
COMMENT ON COLUMN nodes.stake_exra       IS 'Staked EXRA amount. Peak requires >= 100 EXRA. Slashable on fraud.';
COMMENT ON COLUMN nodes.timelock_hours   IS 'Current payout timelock [0..24h]. Anon starts at 24h. Reduced by Decay Boost.';
COMMENT ON COLUMN nodes.timelock_decay_days IS 'Consecutive honest days. Every 7 days → -4h timelock (max 0h).';
COMMENT ON COLUMN nodes.feeder_trust_score IS 'P2P audit trust score [0.000–1.000]. 10% weight in RS_mult calc.';
COMMENT ON COLUMN nodes.rs_mult          IS 'Reputation Score multiplier [0.500–2.000]. Anon hardcapped at 0.500.';
COMMENT ON COLUMN nodes.honest_days_streak IS 'Consecutive honest days without canary/feeder failures.';

COMMENT ON TABLE feeder_assignments IS 'P2P audit — server assigns feeder nodes (GS>300, diff subnet, stake>10) to audit targets.';
COMMENT ON TABLE feeder_reports     IS 'Feeder verdicts. Consensus: 2/3 fraud verdicts → FreezeNode().';
COMMENT ON TABLE canary_tasks       IS 'Server-injected fake tasks (~5% of stream). Fail → GS=0, day credits burned.';
COMMENT ON TABLE did_payout_velocity IS 'One payout per DID per eligible_at window. Anon: 24h, Peak: instant.';
COMMENT ON TABLE sybil_events       IS 'Audit log for Sybil detection (>5 DIDs per IP in 24h).';
COMMENT ON TABLE oracle_batches     IS 'Daily oracle submissions. Consensus = 2/3 matching payload_hash → batch_mint.';
