package models

import (
	"exra/db"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateAnonTax(t *testing.T) {
	anonTax := CalculateAnonTax(100.0, "anon")
	assert.Equal(t, 25.0, anonTax, "Anon should have 25% tax")

	peakTax := CalculateAnonTax(100.0, "peak")
	assert.Equal(t, 0.0, peakTax, "Peak should have 0% tax")
}

func TestGetDecayBoost(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	// exactly 14 streak days -> 14/7 = 2. 2 * 4h = 8h boost.
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(honest_days_streak\), 0\) FROM nodes WHERE did = \$1`).
		WithArgs("did-decay").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(14))

	boost, err := GetDecayBoost("did-decay")
	require.NoError(t, err)
	assert.Equal(t, 8, boost, "14 days streak should yield 8h decay boost")
}

func TestApplyTimelock(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	// Test 1: Peak tier = 0 timelock
	mock.ExpectQuery(`SELECT identity_tier, MAX\(timelock_hours\) FROM nodes WHERE did = \$1`).
		WithArgs("did-peak").
		WillReturnRows(sqlmock.NewRows([]string{"identity_tier", "max"}).AddRow("peak", 24))

	tl, err := ApplyTimelock("did-peak")
	require.NoError(t, err)
	assert.Equal(t, 0, tl)

	// Test 2: Anon tier with 0 streak
	mock.ExpectQuery(`SELECT identity_tier, MAX\(timelock_hours\) FROM nodes WHERE did = \$1`).
		WithArgs("did-anon-0").
		WillReturnRows(sqlmock.NewRows([]string{"identity_tier", "max"}).AddRow("anon", 24))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(honest_days_streak\), 0\) FROM nodes WHERE did = \$1`).
		WithArgs("did-anon-0").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(0))

	tl, err = ApplyTimelock("did-anon-0")
	require.NoError(t, err)
	assert.Equal(t, 24, tl)

	// Test 3: Anon tier with 42 days streak -> max decay
	mock.ExpectQuery(`SELECT identity_tier, MAX\(timelock_hours\) FROM nodes WHERE did = \$1`).
		WithArgs("did-anon-max").
		WillReturnRows(sqlmock.NewRows([]string{"identity_tier", "max"}).AddRow("anon", 24))
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(honest_days_streak\), 0\) FROM nodes WHERE did = \$1`).
		WithArgs("did-anon-max").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(50)) // 50/7 = 7. 7*4 = 28h boost

	tl, err = ApplyTimelock("did-anon-max")
	require.NoError(t, err)
	assert.Equal(t, 0, tl, "Timelock cannot be negative")
}

func TestClaimPayoutAtomic(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	did := "did:peaq:test"
	amountUSD := 10.0

	// Flow:
	mock.ExpectBegin()

	// 1. Velocity check (allow)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM did_payout_velocity`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 2. Fetch tier
	mock.ExpectQuery(`SELECT identity_tier, device_id FROM nodes`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"identity_tier", "device_id"}).AddRow("anon", "dev-1"))

	// 3. Decay Boost (inline)
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(honest_days_streak\), 0\)`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(0))

	// 4. Lock Balance
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(earned_usd\), 0\) FROM node_earnings WHERE device_id IN \(SELECT device_id FROM nodes WHERE did = \$1\) FOR UPDATE`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(50.0))

	// 5. Create Request
	// netAmount = 10 - 2.5(tax) = 7.5
	mock.ExpectQuery(`INSERT INTO payout_requests`).
		WithArgs("dev-1", "wallet-xyz", amountUSD, 7.5).
		WillReturnRows(sqlmock.NewRows([]string{"id", "device_id", "recipient_wallet", "amount_usd", "gas_fee_chain", "storage_fee_chain", "total_fee_chain", "net_amount_usd", "wallet_initialized", "status", "created_at", "updated_at"}).
			AddRow("req-123", "dev-1", "wallet-xyz", amountUSD, 0, 0, 0, 7.5, true, "pending", time.Now(), time.Now()))

	// 6. Deduct balance
	mock.ExpectExec(`INSERT INTO node_earnings`).
		WithArgs("dev-1", -amountUSD).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 7. Insert Velocity Lock
	mock.ExpectExec(`INSERT INTO did_payout_velocity`).
		WithArgs(did, "req-123", amountUSD, 2.5, 7.5, "anon", 24, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	req, err := ClaimPayout(did, amountUSD, "wallet-xyz")
	require.NoError(t, err)
	assert.Equal(t, "req-123", req.ID)
	assert.Equal(t, 7.5, req.NetAmountUSD)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimPayout_VelocityBlocks(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	mock.ExpectBegin()
	// Active lock found
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM did_payout_velocity`).
		WithArgs("did-peaq").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = ClaimPayout("did-peaq", 10.0, "wallet")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "velocity limit")

	assert.NoError(t, mock.ExpectationsWereMet())
}
