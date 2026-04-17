-- Compute Tasks and Assignments
CREATE TABLE IF NOT EXISTS compute_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id UUID REFERENCES buyers(id),
    task_type TEXT NOT NULL, -- 'ai_inference', 'rendering', 'dummy'
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'assigned', 'completed', 'failed'
    requirements JSONB DEFAULT '{}', -- e.g. {"min_ram": 16, "gpu": true, "arch": "amd64"}
    input_url TEXT,
    output_url TEXT,
    reward_usd NUMERIC(10,6) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS task_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID REFERENCES compute_tasks(id),
    node_id UUID REFERENCES nodes(id),
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'completed', 'failed'
    result_hash TEXT,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast task searching
CREATE INDEX IF NOT EXISTS idx_compute_tasks_status ON compute_tasks(status);
CREATE INDEX IF NOT EXISTS idx_task_assignments_task ON task_assignments(task_id);
CREATE INDEX IF NOT EXISTS idx_task_assignments_node ON task_assignments(node_id);
