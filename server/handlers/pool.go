package handlers

import (
	"encoding/json"
	"exra/models"
	"net/http"

	"github.com/gorilla/mux"
)

// POST /api/pools — create a new pool (node auth required)
func CreatePool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID    string `json:"device_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsPublic    *bool  `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.Name == "" {
		jsonError(w, "device_id and name are required", http.StatusBadRequest)
		return
	}
	if err := validateDeviceID(req.DeviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Name) < 3 || len(req.Name) > 50 {
		jsonError(w, "pool name must be 3–50 characters", http.StatusBadRequest)
		return
	}
	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}
	pool, err := models.CreatePool(req.DeviceID, req.Name, req.Description, isPublic)
	if err != nil {
		jsonError(w, "failed to create pool: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, pool, http.StatusCreated)
}

// POST /api/pools/{id}/join
func JoinPool(w http.ResponseWriter, r *http.Request) {
	poolID := mux.Vars(r)["id"]
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || poolID == "" {
		jsonError(w, "device_id and pool id are required", http.StatusBadRequest)
		return
	}
	if err := models.JoinPool(poolID, req.DeviceID); err != nil {
		jsonError(w, "failed to join pool: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pool, _ := models.GetPool(poolID)
	jsonResponse(w, pool, http.StatusOK)
}

// POST /api/pools/leave
func LeavePool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	if err := models.LeavePool(req.DeviceID); err != nil {
		jsonError(w, "not in a pool", http.StatusBadRequest)
		return
	}
	jsonResponse(w, map[string]string{"status": "left"}, http.StatusOK)
}

// GET /api/pools — list all public pools
func ListPools(w http.ResponseWriter, r *http.Request) {
	pools, err := models.ListPools(100)
	if err != nil {
		jsonError(w, "failed to list pools: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if pools == nil {
		pools = []models.Pool{}
	}
	jsonResponse(w, pools, http.StatusOK)
}

// GET /api/pools/me?device_id=xxx — get the pool this node belongs to
func GetMyPool(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	pool, err := models.GetPoolByDevice(nil, deviceID)
	if err != nil {
		jsonError(w, "failed to look up pool: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if pool == nil {
		jsonResponse(w, map[string]any{"pool": nil, "tier": "solo", "treasury_fee_pct": 30}, http.StatusOK)
		return
	}
	jsonResponse(w, pool, http.StatusOK)
}

// GET /api/pools/{id}
func GetPool(w http.ResponseWriter, r *http.Request) {
	poolID := mux.Vars(r)["id"]
	pool, err := models.GetPool(poolID)
	if err != nil {
		jsonError(w, "pool not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, pool, http.StatusOK)
}
