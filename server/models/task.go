package models

import (
	"encoding/json"
	"fmt"
	"exra/db"
	"time"
)

type ComputeTask struct {
	ID           string          `json:"id"`
	BuyerID      string          `json:"buyer_id"`
	TaskType     string          `json:"task_type"`
	Status       string          `json:"status"`
	Requirements json.RawMessage `json:"requirements"`
	MinVRAMMB    int             `json:"min_vram_mb"`
	MinCPUCores  int             `json:"min_cpu_cores"`
	InputURL     string          `json:"input_url,omitempty"`
	OutputURL    string          `json:"output_url,omitempty"`
	RewardUSD    float64         `json:"reward_usd"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type TaskAssignment struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"task_id"`
	NodeID     string    `json:"node_id"`
	Status     string    `json:"status"`
	ResultHash string    `json:"result_hash,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
}

func CreateTask(buyerID, taskType string, requirements interface{}, minVRAM, minCPU int, inputURL string, rewardUSD float64) (*ComputeTask, error) {
	reqJSON, err := json.Marshal(requirements)
	if err != nil {
		return nil, err
	}

	task := &ComputeTask{}
	err = db.DB.QueryRow(
		`INSERT INTO compute_tasks (buyer_id, task_type, requirements, min_vram_mb, min_cpu_cores, input_url, reward_usd)
		 VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7)
		 RETURNING id, buyer_id, task_type, status, requirements, min_vram_mb, min_cpu_cores, COALESCE(input_url, ''), COALESCE(output_url, ''), reward_usd, created_at, updated_at`,
		buyerID, taskType, string(reqJSON), minVRAM, minCPU, inputURL, rewardUSD,
	).Scan(&task.ID, &task.BuyerID, &task.TaskType, &task.Status, &task.Requirements, &task.MinVRAMMB, &task.MinCPUCores, &task.InputURL, &task.OutputURL, &task.RewardUSD, &task.CreatedAt, &task.UpdatedAt)

	return task, err
}

func GetPendingTasks() ([]ComputeTask, error) {
	rows, err := db.DB.Query(`
		SELECT id, buyer_id, task_type, status, requirements, COALESCE(input_url, ''), COALESCE(output_url, ''), reward_usd, created_at, updated_at
		FROM compute_tasks
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []ComputeTask
	for rows.Next() {
		var t ComputeTask
		if err := rows.Scan(&t.ID, &t.BuyerID, &t.TaskType, &t.Status, &t.Requirements, &t.InputURL, &t.OutputURL, &t.RewardUSD, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func GetTaskByID(id string) (*ComputeTask, error) {
	task := &ComputeTask{}
	err := db.DB.QueryRow(`
		SELECT id, buyer_id, task_type, status, requirements, min_vram_mb, min_cpu_cores, COALESCE(input_url, ''), COALESCE(output_url, ''), reward_usd, created_at, updated_at
		FROM compute_tasks
		WHERE id = $1`,
		id,
	).Scan(&task.ID, &task.BuyerID, &task.TaskType, &task.Status, &task.Requirements, &task.MinVRAMMB, &task.MinCPUCores, &task.InputURL, &task.OutputURL, &task.RewardUSD, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func GetSuitableNode(minVRAM, minCPU int) (*Node, error) {
	node := &Node{}
	err := db.DB.QueryRow(`
		SELECT n.id, COALESCE(n.device_id, ''), COALESCE(n.ip, ''), n.address, n.port, n.country, 
		       COALESCE(n.device_type, ''), COALESCE(n.device_tier,'network'), COALESCE(n.is_residential, true), 
		       COALESCE(n.asn_org, ''), COALESCE(n.status, 'online'), COALESCE(n.traffic_bytes, 0), n.bandwidth_mbps, 
		       COALESCE(n.cpu_model,''), COALESCE(n.cpu_cores,0), COALESCE(n.vram_mb,0), COALESCE(n.ram_mb,0),
		       n.active, COALESCE(n.last_seen, NOW()), n.last_heartbeat, n.created_at,
		       COALESCE(n.did, ''), COALESCE(n.identity_tier, 'anon'), COALESCE(n.rs_score, 0.0)
		FROM nodes n
		LEFT JOIN (
		    SELECT node_id, count(*) as active_count
		    FROM task_assignments
		    WHERE ended_at IS NULL
		    GROUP BY node_id
		) s ON s.node_id = n.id
		WHERE n.active = true AND n.status = 'online' 
		  AND n.last_heartbeat > NOW() - INTERVAL '2 minutes'
		  AND n.identity_tier = 'peak'
		  AND n.rs_score >= 500
		  AND (n.vram_mb >= $1 OR $1 = 0)
		  AND (n.cpu_cores >= $2 OR $2 = 0)
		  AND (COALESCE(s.active_count, 0) < GREATEST(1, n.cpu_cores / 2))
		ORDER BY n.rs_score DESC, (n.vram_mb + n.cpu_cores * 100) DESC
		LIMIT 1`,
		minVRAM, minCPU,
	).Scan(&node.ID, &node.DeviceID, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps,
		&node.CPUModel, &node.CPUCores, &node.VRAMMB, &node.RAMMB,
		&node.Active, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt,
		&node.DID, &node.IdentityTier, &node.RSScore)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func AssignTask(taskID, nodeID string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update task status to 'assigned' only if it's currently 'pending'
	res, err := tx.Exec(`UPDATE compute_tasks SET status = 'assigned', updated_at = NOW() WHERE id = $1 AND status = 'pending'`, taskID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("task %s is not pending or does not exist", taskID)
	}

	// Create assignment record
	_, err = tx.Exec(`INSERT INTO task_assignments (task_id, node_id, status) VALUES ($1, $2, 'active')`, taskID, nodeID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func CompleteTask(taskID, nodeID, resultHash, outputURL string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update assignment
	res, err := tx.Exec(`
		UPDATE task_assignments 
		SET status = 'completed', result_hash = $1, ended_at = NOW() 
		WHERE task_id = $2 AND status = 'active'
		  AND node_id = (SELECT id FROM nodes WHERE device_id = $3)`, 
		resultHash, taskID, nodeID, // nodeID here is actually deviceID from the WS handler
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("active assignment for task %s and node %s not found", taskID, nodeID)
	}

	// Update task
	_, err = tx.Exec(`UPDATE compute_tasks SET status = 'completed', output_url = $1, updated_at = NOW() WHERE id = $2`, outputURL, taskID)
	if err != nil {
		return err
	}

	// Reward Flow: Fetch task reward
	var rewardUSD float64
	_ = tx.QueryRow(`SELECT reward_usd FROM compute_tasks WHERE id = $1`, taskID).Scan(&rewardUSD)

	if rewardUSD > 0 {
		// Distribute reward using the unified 3-stream engine (Worker/Referrer/Treasury)
		if _, err := DistributeReward(tx, nodeID, rewardUSD, "compute_task_reward", ""); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// FailTask handles task timeout or rejection: deducts RS score and resets task for re-assignment.
func FailTask(taskID string, reason string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get the worker assigned to this task
	var nodeID string
	err = tx.QueryRow(`
		SELECT node_id
		FROM task_assignments
		WHERE task_id = $1 AND status = 'active'`,
		taskID,
	).Scan(&nodeID)

	if err != nil {
		// No active assignment found, maybe already failed or completed?
		return nil
	}

	// 2. Mark assignment as failed
	_, err = tx.Exec(`UPDATE task_assignments SET status = 'failed', ended_at = NOW() WHERE task_id = $1 AND status = 'active'`, taskID)
	if err != nil {
		return err
	}

	// 3. Penalty: Deduct 50 RS Score from the node (0 floor)
	_, err = tx.Exec(`
		UPDATE nodes 
		SET rs_score = GREATEST(0, rs_score - 50.0), 
		    updated_at = NOW() 
		WHERE id = $1`, nodeID)
	if err != nil {
		return err
	}

	// 4. Log the penalty in identity_events
	_, err = tx.Exec(`
		INSERT INTO identity_events (did, event_type, metadata)
		SELECT did, 'task_failure_penalty', $2::jsonb FROM nodes WHERE id = $1`,
		nodeID, fmt.Sprintf(`{"task_id": "%s", "reason": "%s", "penalty": 50}`, taskID, reason))
	if err != nil {
		return err
	}

	// 5. Reset task to pending for others to pick up
	_, err = tx.Exec(`UPDATE compute_tasks SET status = 'pending', updated_at = NOW() WHERE id = $1`, taskID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// FindExpiredTasks returns task IDs that have been active longer than 10 minutes.
func FindExpiredTasks() ([]string, error) {
	rows, err := db.DB.Query(`
		SELECT task_id 
		FROM task_assignments 
		WHERE status = 'active' 
		  AND started_at < NOW() - INTERVAL '10 minutes'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

