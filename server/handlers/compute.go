package handlers

import (
	"encoding/json"
	"exra/db"
	"exra/metrics"
	"exra/middleware"
	"exra/models"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type submitTaskRequest struct {
	TaskType     string          `json:"task_type"`
	Requirements json.RawMessage `json:"requirements"`
	MinVRAMMB    int             `json:"min_vram_mb"`
	MinCPUCores  int             `json:"min_cpu_cores"`
	InputURL     string          `json:"input_url"`
	RewardUSD    float64         `json:"reward_usd"`
}

type submitResultRequest struct {
	TaskID     string `json:"task_id"`
	ResultHash string `json:"result_hash"`
	OutputURL  string `json:"output_url"`
	Timestamp  string `json:"timestamp"`
	Signature  string `json:"signature"`
}

// POST /api/compute/submit
func SubmitTask(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	if buyer.BalanceUSD <= 0 {
		metrics.ComputeTasksFailed.Inc()
		jsonError(w, "insufficient balance", http.StatusPaymentRequired)
		return
	}

	var req submitTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// For MVP: Auto-select a suitable compute node.
	// Uses a hardware-aware dispatcher.
	node, err := models.GetSuitableNode(req.MinVRAMMB, req.MinCPUCores)
	if err != nil {
		metrics.ComputeTasksFailed.Inc()
		jsonError(w, "no suitable compute nodes available for these requirements", http.StatusServiceUnavailable)
		return
	}
	log.Printf("[Compute] Selected node %s (device_id: %s) for task", node.ID, node.DeviceID)

	task, err := models.CreateTask(buyer.ID, req.TaskType, req.Requirements, req.MinVRAMMB, req.MinCPUCores, req.InputURL, req.RewardUSD)
	if err != nil {
		jsonError(w, "failed to create task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Assign task to node.
	if err := models.AssignTask(task.ID, node.ID); err != nil {
		jsonError(w, "failed to assign task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Push task to the node via WebSocket
	if runtimeHub != nil && node.DeviceID != "" {
		runtimeHub.BroadcastComputeTask(node.DeviceID, task)
	}

	metrics.ComputeTasksSubmitted.Inc()
	jsonResponse(w, task, http.StatusCreated)
}

// GET /api/compute/jobs/{id}
func GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	id := mux.Vars(r)["id"]

	task, err := models.GetTaskByID(id)
	if err != nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
	// Ownership check: buyers can only read their own tasks.
	if task.BuyerID != buyer.ID {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, task, http.StatusOK)
}

// POST /api/compute/node/result — node submits result with DID signature
func SubmitComputeResult(w http.ResponseWriter, r *http.Request) {
	var req submitResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Fetch task and verify existence
	task, err := models.GetTaskByID(req.TaskID)
	if err != nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
	if task.Status != "assigned" {
		jsonError(w, "task is not in assigned state", http.StatusBadRequest)
		return
	}

	// 2. Fetch active assignment and node public key
	var nodeID, pubKey, deviceID string
	err = db.DB.QueryRow(`
		SELECT n.id, n.public_key, n.device_id
		FROM task_assignments ta
		JOIN nodes n ON n.id = ta.node_id
		WHERE ta.task_id = $1 AND ta.status = 'active'`,
		req.TaskID,
	).Scan(&nodeID, &pubKey, &deviceID)

	if err != nil {
		jsonError(w, "active assignment not found for this task", http.StatusNotFound)
		return
	}

	// 3. Verify ZK-light Signature: Sign("$task_id:$result_hash:$timestamp")
	signMsg := req.TaskID + ":" + req.ResultHash + ":" + req.Timestamp
	ok, err := middleware.VerifyDIDSignature(pubKey, signMsg, req.Signature)
	if err != nil || !ok {
		log.Printf("[Compute] Result signature verification failed for task=%s node=%s", req.TaskID, deviceID)
		jsonError(w, "invalid result signature (DID verification failed)", http.StatusForbidden)
		return
	}

	// 4. Complete Task and Distribute Rewards
	if err := models.CompleteTask(req.TaskID, deviceID, req.ResultHash, req.OutputURL); err != nil {
		jsonError(w, "failed to complete task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "accepted", "message": "result verified and reward distributed"}, http.StatusOK)
}

