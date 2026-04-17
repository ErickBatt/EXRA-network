package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestFinalizeSession_DoubleChargePrevention(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	sessionID := "sess-123"
	buyerID := "buyer-456"
	rate := 1.5 // $1.5 per GB

	now := time.Now()
	// Scenario: Session is ALREADY Billed.
	rows := sqlmock.NewRows([]string{"id", "buyer_id", "node_id", "offer_id", "started_at", "ended_at", "bytes_used", "cost_usd", "locked_price_per_gb", "active", "billed"}).
		AddRow(sessionID, buyerID, "node-1", nil, now, &now, 1024*1024*1024, 1.5, 1.5, false, true)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM sessions").WithArgs(sessionID, buyerID).WillReturnRows(rows)
	mock.ExpectCommit()

	sess, charged, err := FinalizeSession(sessionID, buyerID, 0, rate)
	assert.NoError(t, err)
	assert.False(t, charged, "Should not charge an already billed session")
	assert.True(t, sess.Billed)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFinalizeSession_InsufficientBalance(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	sessionID := "sess-low-bal"
	buyerID := "buyer-broke"
	rate := 1.5

	now := time.Now()
	// Session has $1.5 cost, not billed yet.
	rows := sqlmock.NewRows([]string{"id", "buyer_id", "node_id", "offer_id", "started_at", "ended_at", "bytes_used", "cost_usd", "locked_price_per_gb", "active", "billed"}).
		AddRow(sessionID, buyerID, "node-1", nil, now, nil, 1024*1024*1024, 1.5, 1.5, true, false)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM sessions").WithArgs(sessionID, buyerID).WillReturnRows(rows)
	// Active update
	mock.ExpectQuery("UPDATE sessions SET active = false").WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"ended_at", "active"}).AddRow(now, false))
	// Charge attempt: returns 0 affected rows (insufficient balance)
	mock.ExpectExec("UPDATE buyers SET balance_usd = balance_usd - \\$1").
		WithArgs(1.5, buyerID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, charged, err := FinalizeSession(sessionID, buyerID, 0, rate)
	assert.ErrorIs(t, err, ErrInsufficientBuyerBalance)
	assert.False(t, charged)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFinalizeSession_ZeroBytes(t *testing.T) {
	mock, cleanup := setupMockDB(t)
	defer cleanup()

	sessionID := "sess-zero"
	buyerID := "buyer-1"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "buyer_id", "node_id", "offer_id", "started_at", "ended_at", "bytes_used", "cost_usd", "locked_price_per_gb", "active", "billed"}).
		AddRow(sessionID, buyerID, "node-1", nil, now, nil, 0, 0, 1.5, true, false)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM sessions").WithArgs(sessionID, buyerID).WillReturnRows(rows)
	mock.ExpectQuery("UPDATE sessions SET active = false").WithArgs(sessionID).
		WillReturnRows(sqlmock.NewRows([]string{"ended_at", "active"}).AddRow(now, false))
	// cost is 0, so no buyer update expected.
	mock.ExpectExec("UPDATE sessions SET billed = true").WithArgs(sessionID).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	sess, charged, err := FinalizeSession(sessionID, buyerID, 0, 1.5)
	assert.NoError(t, err)
	assert.False(t, charged, "Zero cost session should not result in a charge operation")
	assert.Equal(t, float64(0), sess.CostUSD)

	assert.NoError(t, mock.ExpectationsWereMet())
}
