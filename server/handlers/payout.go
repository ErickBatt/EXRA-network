package handlers

import (
	"encoding/json"
	"exra/models"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// GET /api/node/earnings?device_id=xxx
func GetNodeEarnings(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	earnings, err := models.GetNodeEarnings(deviceID)
	if err != nil {
		jsonError(w, "failed to fetch earnings: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, earnings, http.StatusOK)
}

// POST /api/payout/precheck
func PrecheckPayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID        string  `json:"device_id"`
		AmountUSD       float64 `json:"amount_usd"`
		RecipientWallet string  `json:"recipient_wallet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateDeviceID(req.DeviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateWallet(req.RecipientWallet); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateFloat("amount_usd", req.AmountUSD, 0.000001, 1_000_000); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	summary, err := models.GetNodeEarnings(req.DeviceID)
	if err != nil {
		log.Printf("payout precheck: GetNodeEarnings device=%s err=%v", req.DeviceID, err)
		jsonError(w, "failed to load earnings", http.StatusInternalServerError)
		return
	}
	fees := models.PayoutFeeBreakdown{
		GasFeeChain:     0,
		StorageFeeChain: 0,
		TotalFeeChain:   0,
		WalletReady:     true,
	}
	precheck := models.BuildPayoutPrecheck(req.DeviceID, req.RecipientWallet, req.AmountUSD, summary.TotalUSD, 1.0, fees)
	log.Printf("payout-precheck device=%s amount=%.6f balance=%.6f total_fee_chain=%.9f net=%.6f can=%t", req.DeviceID, req.AmountUSD, summary.TotalUSD, fees.TotalFeeChain, precheck.NetAmountUSD, precheck.CanPayout)
	if !precheck.CanPayout {
		jsonError(w, "Баланса недостаточно для оплаты газа", http.StatusBadRequest)
		return
	}
	jsonResponse(w, precheck, http.StatusOK)
}

// POST /api/payout/request
func RequestPayout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID        string  `json:"device_id"`
		AmountUSD       float64 `json:"amount_usd"`
		RecipientWallet string  `json:"recipient_wallet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.AmountUSD <= 0 || req.RecipientWallet == "" {
		jsonError(w, "invalid payout request: device_id, amount_usd and recipient_wallet are required", http.StatusBadRequest)
		return
	}
	fees := models.PayoutFeeBreakdown{
		GasFeeChain:     0,
		StorageFeeChain: 0,
		TotalFeeChain:   0,
		WalletReady:     true,
	}

	// BuildPayoutPrecheck with a temporary balance (will be re-checked atomically below).
	// We do a quick precheck here just to build the fee breakdown struct.
	tempSummary, err := models.GetNodeEarnings(req.DeviceID)
	if err != nil {
		log.Printf("payout-request: GetNodeEarnings device=%s err=%v", req.DeviceID, err)
		jsonError(w, "failed to load earnings", http.StatusInternalServerError)
		return
	}
	precheck := models.BuildPayoutPrecheck(req.DeviceID, req.RecipientWallet, req.AmountUSD, tempSummary.TotalUSD, 1.0, fees)
	log.Printf("payout-request device=%s wallet=%s amount=%.6f balance=%.6f total_fee_chain=%.9f net=%.6f can=%t", req.DeviceID, req.RecipientWallet, req.AmountUSD, tempSummary.TotalUSD, fees.TotalFeeChain, precheck.NetAmountUSD, precheck.CanPayout)
	if !precheck.CanPayout {
		jsonError(w, "Баланса недостаточно для оплаты газа", http.StatusBadRequest)
		return
	}

	// Atomically re-check balance and create the payout request under a FOR UPDATE lock
	// to prevent TOCTOU double-withdrawal from concurrent requests.
	payout, err := models.CreatePayoutRequestAtomic(precheck)
	if err != nil {
		switch err.Error() {
		case "insufficient earned balance":
			jsonError(w, "insufficient earned balance", http.StatusBadRequest)
		case "payout velocity limit: one withdrawal per 24 hours per device":
			jsonError(w, "you can only request one payout per 24 hours", http.StatusTooManyRequests)
		default:
			log.Printf("payout-request: CreatePayoutRequestAtomic device=%s err=%v", req.DeviceID, err)
			jsonError(w, "failed to create payout request", http.StatusInternalServerError)
		}
		return
	}

	jsonResponse(w, payout, http.StatusCreated)
}

// POST /api/payout/{id}/approve
func ApprovePayout(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		jsonError(w, "payout id is required", http.StatusBadRequest)
		return
	}
	if err := models.UpdatePayoutStatus(id, "approved"); err != nil {
		jsonError(w, "failed to approve payout: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "approved"}, http.StatusOK)
}

// GET /api/payouts
func ListPayouts(w http.ResponseWriter, r *http.Request) {
	out, err := models.ListPayoutRequests(100)
	if err != nil {
		jsonError(w, "failed to list payouts: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = []models.PayoutRequest{}
	}
	jsonResponse(w, out, http.StatusOK)
}

// POST /claim/{did}
// ClaimPayoutHandler processes V2 payouts based on DID. Must be wrapped by
// DIDAuth middleware at registration time; this handler additionally verifies
// that the signed X-DID matches the DID in the URL path so that one node
// cannot claim another's balance.
func ClaimPayoutHandler(w http.ResponseWriter, r *http.Request) {
	did := mux.Vars(r)["did"]
	if did == "" {
		jsonError(w, "did is required in path", http.StatusBadRequest)
		return
	}

	// Defense in depth: DIDAuth has already verified the caller holds the
	// private key matching X-DID. Ensure the path DID matches the authenticated
	// DID so /claim/{other-did} cannot be abused.
	signedDID := r.Header.Get("X-DID")
	if signedDID == "" || signedDID != did {
		jsonError(w, "path DID does not match authenticated DID", http.StatusForbidden)
		return
	}

	var req struct {
		AmountUSD       float64 `json:"amount_usd"`
		RecipientWallet string  `json:"recipient_wallet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateFloat("amount_usd", req.AmountUSD, 0.000001, 1_000_000); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateWallet(req.RecipientWallet); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// For Payout v2, we skip checking gas fees during Phase MVP-3 since Tokenomics v2 dictates
	// they are roughly near 0 and offloaded to Pallet transactions. 
	// We directly call ClaimPayout.
	payout, err := models.ClaimPayout(did, req.AmountUSD, req.RecipientWallet)
	if err != nil {
		switch err.Error() {
		case "insufficient earned balance across DID":
			jsonError(w, "insufficient earned balance", http.StatusBadRequest)
		case "velocity limit: DID has a locked payout window":
			jsonError(w, "you can only request one payout per 24 hours (or while timelocked)", http.StatusTooManyRequests)
		default:
			log.Printf("claim-payout: ClaimPayout did=%s err=%v", did, err)
			jsonError(w, "failed to create claim payout request: " + err.Error(), http.StatusInternalServerError)
		}
		return
	}

	jsonResponse(w, payout, http.StatusCreated)
}
