package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"exra/db"
	"exra/middleware"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTmaWithdraw_AnonTaxAndTimelock(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	deviceID := "dev-123"
	did := "did:peaq:anon"
	amount := 10.0
	const tgID int64 = 42

	// 0. Ownership check runs first: SELECT EXISTS(SELECT 1 FROM tma_devices...)
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM tma_devices`).
		WithArgs(tgID, deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// 1. Mock DID lookup
	mock.ExpectQuery(`SELECT did FROM nodes`).
		WithArgs(deviceID).
		WillReturnRows(sqlmock.NewRows([]string{"did"}).AddRow(did))

	// 2. Mock ClaimPayout logic inside models
	// Step 2.1: DID velocity check
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM did_payout_velocity`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Step 2.2: Fetch tier
	mock.ExpectQuery(`SELECT identity_tier, device_id FROM nodes`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"identity_tier", "device_id"}).AddRow("anon", deviceID))

	// Step 2.3: Decay Boost lookup
	mock.ExpectQuery(`SELECT COALESCE\(MAX\(honest_days_streak\), 0\) FROM nodes`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"streak"}).AddRow(0))

	// Step 2.4: Balance check
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(earned_usd\), 0\) FROM node_earnings`).
		WithArgs(did).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(20.0))

	// Step 2.5: Insert payout_request
	mock.ExpectQuery(`INSERT INTO payout_requests`).
		WithArgs(deviceID, "winning-wallet", amount, 7.5). // Net is 10 - 2.5 = 7.5
		WillReturnRows(sqlmock.NewRows([]string{"id", "device_id", "recipient_wallet", "amount_usd", "gas_fee_chain", "storage_fee_chain", "total_fee_chain", "net_amount_usd", "wallet_initialized", "status", "created_at", "updated_at"}).
			AddRow("pay-1", deviceID, "winning-wallet", amount, 0.0, 0.0, 0.0, 7.5, true, "pending", time.Now(), time.Now()))

	// Step 2.6: Deduct balance
	mock.ExpectExec(`INSERT INTO node_earnings`).
		WithArgs(deviceID, -amount).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Step 2.7: Insert Velocity Lock (24h for anon)
	mock.ExpectExec(`INSERT INTO did_payout_velocity`).
		WithArgs(did, "pay-1", amount, 2.5, 7.5, "anon", 24, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	// 3. Final Velocity lookup for response
	mock.ExpectQuery(`SELECT tax_amount, eligible_at, tier_at_payout FROM did_payout_velocity`).
		WithArgs("pay-1").
		WillReturnRows(sqlmock.NewRows([]string{"tax_amount", "eligible_at", "tier"}).AddRow(2.5, time.Now().Add(24*time.Hour), "anon"))

	// Request Body
	reqBody := map[string]any{
		"device_id":        deviceID,
		"amount_usd":       amount,
		"recipient_wallet": "winning-wallet",
	}
	b, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/api/tma/withdraw", bytes.NewBuffer(b))
	// Inject authenticated TMA session context (middleware is normally responsible).
	req = req.WithContext(context.WithValue(req.Context(), middleware.TMATelegramIDKey, tgID))
	rr := httptest.NewRecorder()

	TmaWithdraw(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var res map[string]any
	json.Unmarshal(rr.Body.Bytes(), &res)

	assert.Equal(t, 2.5, res["tax_amount"])
	assert.Equal(t, "anon", res["tier"])
	assert.Contains(t, res["message"], "anon timelock")
	
	assert.NoError(t, mock.ExpectationsWereMet())
}
