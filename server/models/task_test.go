package models

import (
	"exra/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func setupMockDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}

	oldDB := db.DB
	db.DB = mockDB

	return mock, func() {
		db.DB = oldDB
		mockDB.Close()
	}
}

func TestCreateTask(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	now := time.Now()
	buyerID := "buyer-123"
	taskType := "ai_inference"
	requirements := map[string]interface{}{"gpu": true}
	inputURL := "https://example.com/input.zip"
	reward := 5.0

	// Expectation for QueryRow().Scan()
	rows := sqlmock.NewRows([]string{"id", "buyer_id", "task_type", "status", "requirements", "min_vram_mb", "min_cpu_cores", "input_url", "output_url", "reward_usd", "created_at", "updated_at"}).
		AddRow("task-456", buyerID, taskType, "pending", []byte(`{"gpu":true}`), 0, 0, inputURL, "", reward, now, now)

	mock.ExpectQuery("INSERT INTO compute_tasks").
		WithArgs(buyerID, taskType, `{"gpu":true}`, 0, 0, inputURL, reward).
		WillReturnRows(rows)

	task, err := CreateTask(buyerID, taskType, requirements, 0, 0, inputURL, reward)
	assert.NoError(t, err)
	assert.Equal(t, "task-456", task.ID)
	assert.Equal(t, "pending", task.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAssignTask(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	taskID := "task-1"
	nodeID := "node-a"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE compute_tasks SET status = 'assigned'").
		WithArgs(taskID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO task_assignments").
		WithArgs(taskID, nodeID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := AssignTask(taskID, nodeID)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestComputeTaskLifecycle(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	taskID := "task-life-1"
	nodeID := "node-rich-1"
	reward := 10.0

	// 1. Completion Step
	mock.ExpectBegin()
	// Update assignment (now includes subquery for node_id)
	mock.ExpectExec("UPDATE task_assignments").WithArgs("hash123", taskID, nodeID).WillReturnResult(sqlmock.NewResult(1, 1))
	// Update task
	mock.ExpectExec("UPDATE compute_tasks SET status = 'completed'").WithArgs("url-out", taskID).WillReturnResult(sqlmock.NewResult(1, 1))
	// Reward calculation
	mock.ExpectQuery("SELECT reward_usd FROM compute_tasks").WithArgs(taskID).WillReturnRows(sqlmock.NewRows([]string{"reward_usd"}).AddRow(reward))
	// DistributeReward internal flow
	// Referrer lookup
	mock.ExpectQuery("SELECT COALESCE\\(n.referrer_device_id").WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"referrer_device_id", "referral_count"}).AddRow("", 0))
	// Supply cache (for epoch multiplier) — 0 minted = Genesis epoch (x2.0)
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(amount_exra\\), 0\\) FROM oracle_mint_queue").
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(0))
	// Node hardware lookup (for Gear Score) — empty node = Grade D
	// Anon identity defaults to 0.5 RS Multiplier limit.
	// epoch(x2.0) * anon_cap(x0.5) = x1.0 total multiplier
	mock.ExpectQuery("SELECT COALESCE\\(cpu_cores").WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"cpu_cores", "vram_mb", "ram_mb", "bandwidth_mbps", "device_type", "is_residential", "identity_tier", "feeder_trust_score", "uptime_pct"}).
			AddRow(0, 0, 0, 0, "", false, "anon", 0.5, 99.0))
	
	// Feeder boost lookup
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM feeder_reports").
		WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	
	// Sybil lookup
	mock.ExpectQuery("SELECT COALESCE\\(ip,''\\) FROM nodes").WithArgs(nodeID).
		WillReturnRows(sqlmock.NewRows([]string{"ip"}).AddRow("1.1.1.1"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM nodes").
		WithArgs("1.1.1.%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	// effectiveAmount = reward(10) * multiplier(0.5) = 5.0 (Epoch 1.0 * Anon 0.5 Cap)
	// treasuryFeeRate = 0.25 (anon), treasuryReward = 1.25, workerReward = 3.75
	workerReward := 3.75
	treasuryReward := 1.25
	// 3. Insert reward event
	mock.ExpectQuery("INSERT INTO pop_reward_events.*").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(999)))
	mock.ExpectExec("INSERT INTO treasury_ledger").WithArgs(int64(999), treasuryReward).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO node_earnings").WithArgs(nodeID, workerReward).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO oracle_mint_queue").WithArgs(int64(999), nodeID, workerReward).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE nodes\\s+SET total_worker_reward").WithArgs(workerReward, treasuryReward, nodeID).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := CompleteTask(taskID, nodeID, "hash123", "url-out")
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}
