package handlers

import (
	"encoding/json"
	"errors"
	"exra/middleware"
	"exra/models"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// POST /api/offers
//
// Validation order matters: we decode and validate the body BEFORE
// touching middleware.BuyerFromContext, so a malformed request from an
// unauthenticated path (or a test) gets a precise 400 instead of NPE'ing
// on a nil buyer. The float guards run through validateFloat so NaN /
// +Inf cannot slip past the naive `<= 0` check (NaN <= 0 is false in Go).
func CreateOffer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Country       string  `json:"country"`
		TargetGB      float64 `json:"target_gb"`
		MaxPricePerGB float64 `json:"max_price_per_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := validateCountry(req.Country); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Min 0.0001 GB / $0.0001/GB rejects 0 and negative values without
	// allowing dust-precision overflows. Max bounds preserve the previous
	// guard against absurd numbers being persisted.
	if err := validateFloat("target_gb", req.TargetGB, 0.0001, 100_000); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateFloat("max_price_per_gb", req.MaxPricePerGB, 0.0001, 1_000); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	buyer := middleware.BuyerFromContext(r)
	if buyer == nil {
		jsonError(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	o, err := models.CreateOffer(buyer.ID, req.Country, req.TargetGB, req.MaxPricePerGB)
	if err != nil {
		jsonError(w, "failed to create offer: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, o, http.StatusCreated)
}

// GET /api/offers?limit=20
func ListOffers(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	out, err := models.ListOffersByBuyer(buyer.ID, limit)
	if err != nil {
		jsonError(w, "failed to list offers: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, out, http.StatusOK)
}

// POST /api/offers/{id}/assign
func AssignOffer(w http.ResponseWriter, r *http.Request) {
	offerID := mux.Vars(r)["id"]
	offer, node, session, err := models.AssignOffer(offerID)
	if err != nil {
		if errors.Is(err, models.ErrOfferNotFound) {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, "failed to assign offer: "+err.Error(), http.StatusBadRequest)
		return
	}
	jsonResponse(w, map[string]any{
		"offer":   offer,
		"node":    node,
		"session": session,
	}, http.StatusOK)
}
