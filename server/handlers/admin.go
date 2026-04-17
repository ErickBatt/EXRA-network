package handlers

import (
	"encoding/json"
	"errors"
	"exra/middleware"
	"exra/models"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func adminRequestID(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Request-ID")); v != "" {
		return v
	}
	return uuid.NewString()
}

func writeAdminAudit(r *http.Request, action, resourceType, resourceID, result, errText string, payload map[string]any) {
	actor := middleware.AdminFromContext(r)
	if actor == nil {
		return
	}
	rawPayload, _ := json.Marshal(payload)
	_ = models.InsertAdminAuditLog(models.AdminAuditLog{
		ActorID:         actor.ID,
		ActorEmail:      actor.Email,
		Role:            actor.Role,
		Action:          action,
		ResourceType:    resourceType,
		ResourceID:      resourceID,
		RequestID:       adminRequestID(r),
		IP:              r.RemoteAddr,
		UserAgent:       r.UserAgent(),
		PayloadRedacted: string(rawPayload),
		Result:          result,
		ErrorText:       errText,
	})
}

// GET /api/admin/tokenomics/stats
func AdminTokenomicsStats(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetTokenomicsStats()
	if err != nil {
		jsonError(w, "failed to load tokenomics stats: "+err.Error(), http.StatusInternalServerError)
		return
	}
	p := models.GetPolicy()
	paused, until := models.SwapGuardState()
	epoch := models.CheckCurrentEpoch(stats.TotalExraMinted)
	actor := middleware.AdminFromContext(r)
	jsonResponse(w, map[string]any{
		"request_id":           adminRequestID(r),
		"actor_email":          actor.Email,
		"stats":                stats,
		"max_supply":           p.MaxSupply,
		"epoch_size":           p.EpochSize,
		"policy_finalized":     p.Finalized,
		"swap_circuit_breaker": paused,
		"swap_circuit_until":   until,
		"epoch":                epoch,
	}, http.StatusOK)
}

// GET /api/admin/oracle/queue?limit=100
func AdminOracleQueue(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	items, err := models.ListOracleQueue(limit)
	if err != nil {
		jsonError(w, "failed to load oracle queue: "+err.Error(), http.StatusInternalServerError)
		return
	}
	actor := middleware.AdminFromContext(r)
	jsonResponse(w, map[string]any{
		"request_id":  adminRequestID(r),
		"actor_email": actor.Email,
		"items":       items,
	}, http.StatusOK)
}

// POST /api/admin/oracle/queue/{id}/retry
func AdminRetryOracleQueueItem(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "invalid queue id", http.StatusBadRequest)
		return
	}
	if err := models.RetryOracleMintNow(id); err != nil {
		writeAdminAudit(r, "oracle.retry", "oracle_mint_queue", idStr, "error", err.Error(), map[string]any{"id": id})
		jsonError(w, "failed to retry queue item: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeAdminAudit(r, "oracle.retry", "oracle_mint_queue", idStr, "success", "", map[string]any{"id": id})
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"status":     "queued",
		"id":         id,
	}, http.StatusOK)
}

// POST /api/admin/oracle/process
func AdminProcessOracleQueue(w http.ResponseWriter, r *http.Request) {
	// TON legacy process removed. In PEAQ, batch_mint happens in models.RunOracleWorker()
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"processed":  0,
		"message":   "PEAQ oracle worker is running in background. Use /api/admin/peaq/trigger-batch for manual trigger.",
	}, http.StatusOK)
}

// POST /api/admin/peaq/trigger-batch
func AdminTriggerPeaqBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BatchDate   string `json:"batch_date"`
		PayloadHash string `json:"payload_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BatchDate == "" || req.PayloadHash == "" {
		jsonError(w, "batch_date and payload_hash are required", http.StatusBadRequest)
		return
	}

	// Trigger async
	go models.TriggerBatchMint(req.BatchDate, req.PayloadHash)

	actor := middleware.AdminFromContext(r)
	writeAdminAudit(r, "peaq.trigger_batch", "oracle_batches", req.PayloadHash, "success", "", map[string]any{
		"batch_date": req.BatchDate,
		"hash":       req.PayloadHash,
	})

	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"actor":      actor.Email,
		"status":     "triggered",
		"message":    "Peaq batch minting process started in background. Check server logs for transaction hash.",
	}, http.StatusOK)
}

// GET /api/admin/payouts
func AdminListPayouts(w http.ResponseWriter, r *http.Request) {
	out, err := models.ListPayoutRequests(200)
	if err != nil {
		jsonError(w, "failed to list payouts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []models.PayoutRequest{}
	}
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"items":      out,
	}, http.StatusOK)
}

// POST /api/admin/payout/{id}/approve
func AdminApprovePayout(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		jsonError(w, "payout id is required", http.StatusBadRequest)
		return
	}
	if err := models.UpdatePayoutStatus(id, "approved"); err != nil {
		writeAdminAudit(r, "payout.approve", "payout_requests", id, "error", err.Error(), nil)
		jsonError(w, "failed to approve payout: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeAdminAudit(r, "payout.approve", "payout_requests", id, "success", "", nil)
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"status":     "approved",
	}, http.StatusOK)
}

// POST /api/admin/payout/{id}/mark-paid
//
// Marks a previously approved payout as fulfilled, recording the off-ramp
// transaction hash so the user has a verifiable receipt. The DB constraint
// rejects rows in any other status, and `MarkPayoutPaid` translates the
// race-safe UPDATE result into a typed error so we can pick the right HTTP
// code instead of leaking a generic 500.
func AdminMarkPayoutPaid(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		jsonError(w, "payout id is required", http.StatusBadRequest)
		return
	}
	var req struct {
		TxHash   string `json:"tx_hash"`
		Provider string `json:"provider"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TxHash == "" {
		jsonError(w, "tx_hash is required", http.StatusBadRequest)
		return
	}
	if len(req.TxHash) > 256 || len(req.Provider) > 64 || len(req.Note) > 1024 {
		jsonError(w, "tx_hash, provider or note exceeds maximum length", http.StatusBadRequest)
		return
	}

	updated, err := models.MarkPayoutPaid(id, req.TxHash, req.Provider, req.Note)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrPayoutNotFound):
			writeAdminAudit(r, "payout.mark_paid", "payout_requests", id, "error", "not found", nil)
			jsonError(w, "payout not found", http.StatusNotFound)
		case errors.Is(err, models.ErrPayoutNotApproved):
			writeAdminAudit(r, "payout.mark_paid", "payout_requests", id, "error", "not approved", nil)
			jsonError(w, err.Error(), http.StatusConflict)
		default:
			writeAdminAudit(r, "payout.mark_paid", "payout_requests", id, "error", err.Error(), nil)
			jsonError(w, "failed to mark payout paid: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeAdminAudit(r, "payout.mark_paid", "payout_requests", id, "success", "tx="+req.TxHash, nil)
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"payout":     updated,
	}, http.StatusOK)
}

// POST /api/admin/payout/{id}/reject
func AdminRejectPayout(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		jsonError(w, "payout id is required", http.StatusBadRequest)
		return
	}
	if err := models.UpdatePayoutStatus(id, "rejected"); err != nil {
		writeAdminAudit(r, "payout.reject", "payout_requests", id, "error", err.Error(), nil)
		jsonError(w, "failed to reject payout: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeAdminAudit(r, "payout.reject", "payout_requests", id, "success", "", nil)
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"status":     "rejected",
	}, http.StatusOK)
}

// GET /api/admin/incidents
func AdminIncidents(w http.ResponseWriter, r *http.Request) {
	summary, err := models.GetAdminIncidentSummary()
	if err != nil {
		jsonError(w, "failed to load incidents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{
		"request_id": adminRequestID(r),
		"summary":    summary,
	}, http.StatusOK)
}

// POST /api/admin/node/freeze — permanently freeze a node (anti-fraud)
func AdminFreezeNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceID == "" {
		jsonError(w, "device_id required", http.StatusBadRequest)
		return
	}
	if req.Reason == "" {
		req.Reason = "admin_manual_freeze"
	}
	if err := models.FreezeNode(req.DeviceID, req.Reason); err != nil {
		writeAdminAudit(r, "node.freeze", "nodes", req.DeviceID, "error", err.Error(), nil)
		jsonError(w, "failed to freeze node: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeAdminAudit(r, "node.freeze", "nodes", req.DeviceID, "success", "", map[string]any{"reason": req.Reason})
	jsonResponse(w, map[string]any{"status": "frozen", "device_id": req.DeviceID}, http.StatusOK)
}

// GET /api/admin/circuit-breaker — current mint circuit breaker state
func AdminCircuitBreakerState(w http.ResponseWriter, r *http.Request) {
	paused, until, reason := models.MintCircuitBreakerState()
	jsonResponse(w, map[string]any{
		"paused":    paused,
		"until":     until,
		"reason":    reason,
	}, http.StatusOK)
}
