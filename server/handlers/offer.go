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
func CreateOffer(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	var req struct {
		Country       string  `json:"country"`
		TargetGB      float64 `json:"target_gb"`
		MaxPricePerGB float64 `json:"max_price_per_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.TargetGB <= 0 || req.MaxPricePerGB <= 0 {
		jsonError(w, "target_gb and max_price_per_gb must be > 0", http.StatusBadRequest)
		return
	}
	if req.TargetGB > 100_000 || req.MaxPricePerGB > 1_000 {
		jsonError(w, "target_gb or max_price_per_gb exceeds allowed maximum", http.StatusBadRequest)
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
