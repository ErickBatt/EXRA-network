package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// --- CreatePool ---

func TestCreatePool_RejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/pools", strings.NewReader("{{{"))
	rr := httptest.NewRecorder()
	CreatePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreatePool_RejectsMissingDeviceID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"name": "MyPool"})
	req := httptest.NewRequest("POST", "/api/pools", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	CreatePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "device_id")
}

func TestCreatePool_RejectsMissingName(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"device_id": "dev-1"})
	req := httptest.NewRequest("POST", "/api/pools", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	CreatePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "name")
}

func TestCreatePool_RejectsShortName(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"device_id": "dev-1", "name": "ab"})
	req := httptest.NewRequest("POST", "/api/pools", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	CreatePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "3")
}

func TestCreatePool_RejectsLongName(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"device_id": "dev-1",
		"name":      strings.Repeat("a", 51),
	})
	req := httptest.NewRequest("POST", "/api/pools", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	CreatePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreatePool_RejectsOversizedDeviceID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"device_id": strings.Repeat("d", 129),
		"name":      "GoodName",
	})
	req := httptest.NewRequest("POST", "/api/pools", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	CreatePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "device_id")
}

// --- JoinPool ---

func TestJoinPool_RejectsMissingDeviceID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{})
	rr := withMuxVars(
		"POST",
		"/api/pools/p1/join",
		"/api/pools/{id}/join",
		body,
		nil,
		JoinPool,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "device_id")
}

func TestJoinPool_RejectsInvalidJSON(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/api/pools/{id}/join", JoinPool).Methods("POST")
	req := httptest.NewRequest("POST", "/api/pools/p1/join", strings.NewReader("xxx"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// --- LeavePool ---

func TestLeavePool_RejectsMissingDeviceID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("POST", "/api/pools/leave", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	LeavePool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "device_id")
}

// --- GetMyPool ---

func TestGetMyPool_RejectsMissingDeviceID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/pools/me", nil)
	rr := httptest.NewRecorder()
	GetMyPool(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
