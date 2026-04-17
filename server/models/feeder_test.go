package models

import (
	"exra/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignFeeder_SubnetLock(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	targetID := "target-node"
	targetIP := "192.168.1.10"
	targetSubnet := "192.168.1."

	// 1. Get target IP
	mock.ExpectQuery(`SELECT COALESCE\(ip, ''\) FROM nodes WHERE device_id = \$1`).
		WithArgs(targetID).
		WillReturnRows(sqlmock.NewRows([]string{"ip"}).AddRow(targetIP))

	// 2. Query for feeder - must exclude targetSubnet
	feederID := "feeder-node"
	feederIP := "203.0.113.5"
	mock.ExpectQuery(`SELECT device_id, ip FROM nodes`).
		WithArgs(targetID, targetSubnet+"%").
		WillReturnRows(sqlmock.NewRows([]string{"device_id", "ip"}).AddRow(feederID, feederIP))

	// 3. Insert assignment
	mock.ExpectQuery(`INSERT INTO feeder_assignments`).
		WithArgs(targetID, feederID, targetSubnet, "203.0.113.").
		WillReturnRows(sqlmock.NewRows([]string{"id", "assigned_at", "expires_at", "status"}).
			AddRow(int64(100), time.Now(), time.Now().Add(time.Hour), "pending"))

	asgn, err := AssignFeeder(targetID)
	require.NoError(t, err)
	assert.Equal(t, feederID, asgn.FeederDeviceID)
	assert.Equal(t, "pending", asgn.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEvaluateFeederConsensus_FraudMajority(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	targetID := "fraud-node"

	// 1. Fetch 3 reports: 2 fraud, 1 honest (66% fraud)
	mock.ExpectQuery(`SELECT id, feeder_device_id, verdict FROM feeder_reports`).
		WithArgs(targetID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feeder_device_id", "verdict"}).
			AddRow(1, "f1", "fraud").
			AddRow(2, "f2", "fraud").
			AddRow(3, "f3", "honest"))

	// 2. Freeze the target
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE nodes`).
		WithArgs(targetID, "feeder_consensus_fraud").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO node_earnings`).
		WithArgs(targetID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE payout_requests`).
		WithArgs(targetID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE oracle_mint_queue`).
		WithArgs(targetID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// 3. Process reports (Deterministically sorted by ID: 1, 2, 3)
	// ID 1 (Correct)
	mock.ExpectExec(`UPDATE feeder_reports SET evaluated_at = NOW\(\), evaluation_result = \$1 WHERE id = \$2`).
		WithArgs("correct", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE feeder_assignments SET status = 'evaluated'`).WillReturnResult(sqlmock.NewResult(0, 1))
	
	// ID 2 (Correct)
	mock.ExpectExec(`UPDATE feeder_reports SET evaluated_at = NOW\(\), evaluation_result = \$1 WHERE id = \$2`).
		WithArgs("correct", int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE feeder_assignments SET status = 'evaluated'`).WillReturnResult(sqlmock.NewResult(0, 1))

	// ID 3 (Incorrect - Slashing)
	mock.ExpectExec(`UPDATE nodes SET stake_exra = stake_exra \* \(1.0 - \$2\)`).
		WithArgs("f3", 0.05).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE feeder_reports SET evaluated_at = NOW\(\), evaluation_result = \$1 WHERE id = \$2`).
		WithArgs("false_negative", int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE feeder_assignments SET status = 'evaluated'`).WillReturnResult(sqlmock.NewResult(0, 1))

	EvaluateFeederConsensus(targetID)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSlashFeeder_Deduction(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	feederID := "bad-feeder"
	
	mock.ExpectExec(`UPDATE nodes SET stake_exra = stake_exra \* \(1.0 - \$2\)`).
		WithArgs(feederID, 0.05).
		WillReturnResult(sqlmock.NewResult(0, 1))

	SlashFeeder(feederID, 0.05)

	assert.NoError(t, mock.ExpectationsWereMet())
}
