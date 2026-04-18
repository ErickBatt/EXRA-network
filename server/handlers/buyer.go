package handlers

import (
	"encoding/json"
	"exra/middleware"
	"exra/models"
	"log"
	"net/http"
)

// POST /api/buyer/register
// Protected by ProxySecret (admin endpoint to create buyers)
func RegisterBuyer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateEmail(req.Email); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	buyer, err := models.CreateBuyer(req.Email)
	if err != nil {
		log.Printf("RegisterBuyer: create failed email=%s err=%v", req.Email, err)
		jsonError(w, "failed to create buyer", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, buyer, http.StatusCreated)
}

// POST /api/buyer/sync
// Takes email and returns buyer (creates if not exists)
func SyncBuyer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateEmail(req.Email); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	buyer, err := models.GetBuyerByEmail(req.Email)
	if err != nil {
		// Create if not exists
		buyer, err = models.CreateBuyer(req.Email)
		if err != nil {
			jsonError(w, "failed to sync buyer: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	jsonResponse(w, buyer, http.StatusOK)
}

// GET /api/buyer/me
func GetBuyerProfile(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	jsonResponse(w, buyer, http.StatusOK)
}

// GET /api/buyer/sessions
func GetBuyerSessions(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	sessions, err := models.GetBuyerSessions(buyer.ID, 50)
	if err != nil {
		jsonError(w, "failed to get sessions: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []models.Session{}
	}
	jsonResponse(w, sessions, http.StatusOK)
}

const maxTopUpUSD = 10_000.0 // single top-up cap to prevent overflow / accidental over-crediting

// POST /api/buyer/topup
//
// Validation order: parse + validate body BEFORE reading the buyer from
// context, so a malformed payload (or a bypassed-auth path) returns a
// clean 400 instead of NPE'ing on a nil buyer. validateFloat closes the
// NaN/Inf hole the previous `<= 0` check missed (NaN <= 0 is false).
func TopUpBalance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Amount float64 `json:"amount_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateFloat("amount_usd", req.Amount, 0.01, maxTopUpUSD); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	buyer := middleware.BuyerFromContext(r)
	if buyer == nil {
		jsonError(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	if err := models.TopUpBalance(buyer.ID, req.Amount); err != nil {
		jsonError(w, "topup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]interface{}{"status": "ok", "added_usd": req.Amount}, http.StatusOK)
}

