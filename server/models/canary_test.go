package models

import (
	"exra/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanaryInjectionRate(t *testing.T) {
	injections := 0
	runs := 10000
	for i := 0; i < runs; i++ {
		if ShouldInjectCanary("node-test") {
			injections++
		}
	}
	rate := float64(injections) / float64(runs)
	// 5% ± 1% is between 4% and 6% (0.04 to 0.06)
	assert.GreaterOrEqual(t, rate, 0.04)
	assert.LessOrEqual(t, rate, 0.06)
}

func TestCreateCanaryTask(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	deviceID := "dev-1"

	// Existing pending canary found (blocks creating new one)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM canary_tasks`).
		WithArgs(deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	_, err = CreateCanaryTask(deviceID)
	require.Error(t, err)
	assert.Equal(t, "active canary task already exists", err.Error())

	// No pending canary found, allows creating
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM canary_tasks`).
		WithArgs(deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO canary_tasks`).
		WithArgs(deviceID, "canary_expected_hash").
		WillReturnRows(sqlmock.NewRows([]string{"id", "device_id", "task_type", "expected_result", "result", "injected_at"}).
			AddRow(int64(42), deviceID, "proxy_hash", "canary_expected_hash", "pending", now))

	task, err := CreateCanaryTask(deviceID)
	require.NoError(t, err)
	assert.Equal(t, int64(42), task.ID)
	assert.Equal(t, "proxy_hash", task.TaskType)
}

func TestVerifyCanaryResult(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	deviceID := "dev-auth"
	taskID := int64(42)
	expectedHash := "canary_expected_hash"

	// 1. Pass Case
	mock.ExpectQuery(`SELECT expected_result FROM canary_tasks`).
		WithArgs(taskID, deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"expected_result"}).AddRow(expectedHash))

	mock.ExpectExec(`UPDATE canary_tasks SET result = \$1`).
		WithArgs("pass", expectedHash, taskID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	pass := VerifyCanaryResult(deviceID, taskID, expectedHash)
	assert.True(t, pass)

	// 2. Fail Case (BurnDayCredits invoked)
	mock.ExpectQuery(`SELECT expected_result FROM canary_tasks`).
		WithArgs(taskID, deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"expected_result"}).AddRow(expectedHash))

	mock.ExpectExec(`UPDATE canary_tasks SET result = \$1`).
		WithArgs("fail", "wrong_hash", taskID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// BurnDayCredits queries
	mock.ExpectBegin()
	mock.ExpectQuery(`WITH burned AS`).
		WithArgs(deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"earned_usd"}).AddRow(0.5))
	mock.ExpectExec(`UPDATE nodes SET honest_days_streak = 0, rs_mult = 0`).
		WithArgs(deviceID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE canary_tasks SET penalty_applied = true`).
		WithArgs(deviceID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	fail := VerifyCanaryResult(deviceID, taskID, "wrong_hash")
	assert.False(t, fail)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCanaryNotRepeated(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	deviceID := "dev-repeat"

	// Mock that there is already an active (pending) canary task
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM canary_tasks`).
		WithArgs(deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Attempt to create another one should fail
	task, err := CreateCanaryTask(deviceID)
	assert.Error(t, err)
	assert.Nil(t, task)
	assert.Equal(t, "active canary task already exists", err.Error())

	assert.NoError(t, mock.ExpectationsWereMet())
}
