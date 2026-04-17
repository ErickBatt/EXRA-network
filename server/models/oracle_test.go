package models

import (
	"exra/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashDistribution_Deterministic(t *testing.T) {
	dist1 := map[string]float64{
		"did:1": 10.5,
		"did:2": 20.0,
	}
	dist2 := map[string]float64{
		"did:2": 20.0,
		"did:1": 10.5,
	}

	hash1 := HashDistribution(dist1)
	hash2 := HashDistribution(dist2)

	assert.Equal(t, hash1, hash2, "Hashing must be independent of map iteration order")
}

func TestCalculateDailyDistribution_JoinNodes(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	targetDate := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	dateStr := targetDate.Format("2006-01-02")

	rows := sqlmock.NewRows([]string{"did", "sum"}).
		AddRow("did:alice", 10.0).
		AddRow("did:bob", 5.5)

	mock.ExpectQuery(`SELECT n.did, SUM\(e.earned_usd\) FROM node_earnings e`).
		WithArgs(dateStr).
		WillReturnRows(rows)

	dist, err := CalculateDailyDistribution(targetDate)
	require.NoError(t, err)
	assert.Len(t, dist, 2)
	assert.Equal(t, 10.0, dist["did:alice"])
	assert.Equal(t, 5.5, dist["did:bob"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckOracleConsensus_Thresholds(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	batchDate := "2026-04-15"
	batchHash := "final_hash_123"

	// 1. Threshold test: 2/3 of 3 = 2
	// Scenario: 2 proposals found
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oracle_batches`).
		WithArgs(batchDate, batchHash).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// Expect state update to 'consensus'
	mock.ExpectExec(`UPDATE oracle_batches SET status = 'consensus'`).
		WithArgs(batchDate, batchHash).
		WillReturnResult(sqlmock.NewResult(0, 1))

	batchID := int64(12345)
	// Expect lookup of consensus batch for finalization
	mock.ExpectQuery(`SELECT id FROM oracle_batches`).
		WithArgs(batchDate, batchHash).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(batchID))

	// Expect batch_id tagging in earnings
	mock.ExpectExec(`UPDATE node_earnings SET batch_id = \$1`).
		WithArgs(batchID, batchDate).
		WillReturnResult(sqlmock.NewResult(0, 10))

	// Expect 'applied' update (from mock mint trigger)
	mock.ExpectExec(`UPDATE oracle_batches SET status = 'applied'`).
		WithArgs(batchID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	CheckOracleConsensus(batchDate, batchHash, 3)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckOracleConsensus_Insufficient(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	batchDate := "2026-04-15"
	batchHash := "waiting_hash"

	// Scenario: only 1 proposal found, 2/3 of 3 required (2)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oracle_batches`).
		WithArgs(batchDate, batchHash).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// No Exec expected

	CheckOracleConsensus(batchDate, batchHash, 3)

	assert.NoError(t, mock.ExpectationsWereMet())
}
