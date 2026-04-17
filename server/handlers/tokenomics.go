package handlers

import (
	"encoding/json"
	"exra/models"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// GET /api/audit/mints — public mint audit log (no auth required)
// Returns confirmed/minted entries only. Device IDs are truncated for privacy.
func PublicMintAudit(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	entries, err := models.ListPublicMintAudit(limit)
	if err != nil {
		jsonError(w, "failed to load audit log", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{
		"mints":           entries,
		"total":           len(entries),
		"peaq_explorer":   "https://peaq.subscan.io/",
	}, http.StatusOK)
}

// POST /api/tokenomics/oracle/process
func ProcessOracleQueue(w http.ResponseWriter, r *http.Request) {
	items, err := models.ListPendingOracleMints(100)
	if err != nil {
		jsonError(w, "failed to load oracle queue: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{"queued": len(items), "note": "items will be processed by the background oracle worker"}, http.StatusOK)
}

// POST /api/tokenomics/payments/settle
func SettleBuyerPayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuyerID       string  `json:"buyer_id"`
		InputCurrency string  `json:"input_currency"` // USDT or Exra
		InputAmount   float64 `json:"input_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.InputAmount <= 0 {
		jsonError(w, "input_amount must be > 0", http.StatusBadRequest)
		return
	}
	currency := strings.ToUpper(req.InputCurrency)
	if currency != "USDT" && currency != "Exra" {
		jsonError(w, "input_currency must be USDT or Exra", http.StatusBadRequest)
		return
	}

	// Simplified market loop policy.
	ExraBought := req.InputAmount
	if currency == "USDT" {
		ExraBought = req.InputAmount // 1:1 temporary reference pricing.
	}
	ExraBurned := ExraBought * 0.10
	if err := models.RecordBurnEvent(req.BuyerID, currency, req.InputAmount, ExraBought, ExraBurned); err != nil {
		jsonError(w, "failed to record burn event: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{
		"exra_bought": ExraBought,
		"exra_burned": ExraBurned,
		"burn_rate":   0.10,
	}, http.StatusOK)
}

// GET /api/tokenomics/stats
func GetTokenomicsStats(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetTokenomicsStats()
	if err != nil {
		jsonError(w, "failed to load tokenomics stats: "+err.Error(), http.StatusInternalServerError)
		return
	}
	p := models.GetPolicy()
	paused, until := models.SwapGuardState()
	epoch := models.CheckCurrentEpoch(stats.TotalExraMinted)
	jsonResponse(w, map[string]any{
		"stats":                stats,
		"max_supply":           p.MaxSupply,
		"epoch_size":           p.EpochSize,
		"policy_finalized":     p.Finalized,
		"swap_circuit_breaker": paused,
		"swap_circuit_until":   until,
		"epoch":                epoch,
	}, http.StatusOK)
}

// GET /api/tokenomics/epoch — public endpoint for FOMO counter & epoch state
func GetEpochState(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetTokenomicsStats()
	if err != nil {
		jsonError(w, "failed to load stats", http.StatusInternalServerError)
		return
	}
	dailyRate := models.GetAvgDailyMintRate()
	epoch := models.CheckCurrentEpochWithRate(stats.TotalExraMinted, dailyRate)
	jsonResponse(w, epoch, http.StatusOK)
}

// POST /api/tokenomics/swap/quote
func RequestSwapQuote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID   string  `json:"device_id"`
		ExraAmount float64 `json:"exra_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	// 1:1 price for MVP simulation, or fetch from market
	price := 1.0

	rawUSD := req.ExraAmount * price
	spread := rawUSD * 0.10
	received := rawUSD - spread

	jsonResponse(w, models.SwapQuote{
		DeviceID:      req.DeviceID,
		ExraAmount:    req.ExraAmount,
		UsdcReceived:  received,
		SpreadUSD:     spread,
		TreasuryFloor: false,
	}, http.StatusOK)
}

// POST /api/tokenomics/swap/execute
func ExecuteSwap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID   string  `json:"device_id"`
		ExraAmount float64 `json:"exra_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	price := 1.0
	quote, err := models.ExecuteSwap(req.DeviceID, req.ExraAmount, price)
	if err != nil {
		jsonError(w, "swap failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if quote.TreasuryFloor {
		jsonResponse(w, map[string]any{
			"status": "rejected",
			"reason": "Treasury balance too low (Liquidity Floor reached)",
		}, http.StatusLocked)
		return
	}

	jsonResponse(w, quote, http.StatusOK)
}

// GET /api/tokenomics/oracle/queue?limit=100
func GetOracleQueue(w http.ResponseWriter, r *http.Request) {
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
	jsonResponse(w, items, http.StatusOK)
}

// POST /api/tokenomics/oracle/queue/{id}/retry
func RetryOracleQueueItem(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "invalid queue id", http.StatusBadRequest)
		return
	}
	if err := models.RetryOracleMintNow(id); err != nil {
		jsonError(w, "failed to retry queue item: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{"status": "queued", "id": id}, http.StatusOK)
}
