package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// All these tests target the validation layer that runs BEFORE
// touching the DB/middleware, so they're DB-free. The unauthenticated
// (no buyer in context) cases also assert the handler does not NPE.

func postJSON(h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// --- SyncBuyer ---

func TestSyncBuyer_RejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/buyer/sync", strings.NewReader("{not json"))
	rr := httptest.NewRecorder()
	SyncBuyer(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSyncBuyer_RejectsMissingEmail(t *testing.T) {
	rr := postJSON(SyncBuyer, "/api/buyer/sync", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "email")
}

func TestSyncBuyer_RejectsBadEmail(t *testing.T) {
	rr := postJSON(SyncBuyer, "/api/buyer/sync", map[string]any{"email": "no-at-sign"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "email")
}

// --- TopUpBalance ---

func TestTopUpBalance_RejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/buyer/topup", strings.NewReader("{nope"))
	rr := httptest.NewRecorder()
	TopUpBalance(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestTopUpBalance_RejectsZero(t *testing.T) {
	rr := postJSON(TopUpBalance, "/api/buyer/topup", map[string]any{"amount_usd": 0.0})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "amount_usd")
}

func TestTopUpBalance_RejectsNegative(t *testing.T) {
	rr := postJSON(TopUpBalance, "/api/buyer/topup", map[string]any{"amount_usd": -5.0})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "amount_usd")
}

func TestTopUpBalance_RejectsOverCap(t *testing.T) {
	rr := postJSON(TopUpBalance, "/api/buyer/topup", map[string]any{"amount_usd": 10_001.0})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "amount_usd")
}

// Defence in depth: a clean body with no buyer in context must NOT
// dereference nil; it should return 401, not 500.
func TestTopUpBalance_RejectsUnauthenticated(t *testing.T) {
	rr := postJSON(TopUpBalance, "/api/buyer/topup", map[string]any{"amount_usd": 10.0})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// --- RegisterBuyer ---

func TestRegisterBuyer_RejectsBadEmail(t *testing.T) {
	rr := postJSON(RegisterBuyer, "/api/buyer/register", map[string]any{"email": "bad"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "email")
}

func TestRegisterBuyer_RejectsMissingEmail(t *testing.T) {
	rr := postJSON(RegisterBuyer, "/api/buyer/register", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
