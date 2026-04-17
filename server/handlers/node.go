package handlers

import (
	"encoding/json"
	"exra/models"
	"fmt"
	"net/http"
	"strings"
)

type setReferrerRequest struct {
	DeviceID         string `json:"device_id"`
	ReferrerDeviceID string `json:"referrer_device_id"`
}

// POST /api/node/set-referrer
// Binds a node to its referrer (one-time, idempotent).
func SetReferrerHandler(w http.ResponseWriter, r *http.Request) {
	var req setReferrerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.ReferrerDeviceID == "" {
		jsonError(w, "device_id and referrer_device_id are required", http.StatusBadRequest)
		return
	}
	if err := models.SetReferrer(req.DeviceID, req.ReferrerDeviceID); err != nil {
		jsonError(w, "set referrer failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

type registerNodeRequest struct {
	DeviceID      string `json:"device_id"`
	PublicKey     string `json:"public_key"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Country       string `json:"country"`
	BandwidthMbps int    `json:"bandwidth_mbps"`
	DID           string `json:"did"`
}

// POST /api/node/register
func RegisterNode(w http.ResponseWriter, r *http.Request) {
	var req registerNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.PublicKey == "" {
		jsonError(w, "device_id and public_key are required for PEAQ DID identity", http.StatusBadRequest)
		return
	}

	// Internal registration logic now persists the public key immediately
	node, err := models.UpsertWSNode(req.DeviceID, req.PublicKey, r.RemoteAddr, req.Country, "http-node", "network", "", true, "", 0, 0, 0, true, 1.50, req.DID)
	if err != nil {
		jsonError(w, "failed to register node: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, node, http.StatusCreated)
}

// POST /api/node/heartbeat
func NodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	// Identity is now taken from the DIDAuth header, not the URL.
	// This prevents one node from spoofing heartbeats for another.
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		jsonError(w, "unauthorized: missing device identity", http.StatusUnauthorized)
		return
	}

	if err := models.HeartbeatPoP(deviceID, models.GetPopEmission()); err != nil {
		jsonError(w, "heartbeat failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// GET /api/nodes — buyer-facing list, also strips IP for privacy.
func ListNodes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sortBy := strings.TrimSpace(query.Get("sort"))

	filter := models.NodeFilter{
		Country:      strings.TrimSpace(query.Get("country")),
		Tier:         strings.TrimSpace(query.Get("tier")),
		IdentityTier: strings.TrimSpace(query.Get("identity_tier")),
	}

	if vram := query.Get("min_vram"); vram != "" {
		fmt.Sscanf(vram, "%d", &filter.MinVRAM)
	}
	if price := query.Get("max_price"); price != "" {
		fmt.Sscanf(price, "%f", &filter.MaxPrice)
	}

	nodes, err := models.GetActiveNodesWithFilters(sortBy, filter)
	if err != nil {
		jsonError(w, "failed to list nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pub := make([]models.PublicNode, 0, len(nodes))
	for _, n := range nodes {
		pub = append(pub, models.PublicNode{
			ID:            n.ID,
			DID:           n.DID,
			IdentityTier:  n.IdentityTier,
			Country:       n.Country,
			DeviceType:    n.DeviceType,
			DeviceTier:    n.DeviceTier,
			IsResidential: n.IsResidential,
			Status:        n.Status,
			BandwidthMbps: n.BandwidthMbps,
			CPUCores:      n.CPUCores,
			VRAMMB:        n.VRAMMB,
			RAMMB:         n.RAMMB,
			PricePerGB:    n.PricePerGB,
			LastSeen:      n.LastSeen,
		})
	}
	jsonResponse(w, pub, http.StatusOK)
}

// GET /api/nodes/market-price?country=IN
func GetMarketPrice(w http.ResponseWriter, r *http.Request) {
	country := strings.TrimSpace(r.URL.Query().Get("country"))
	if country == "" {
		jsonError(w, "country is required", http.StatusBadRequest)
		return
	}
	price, err := models.GetMarketAvgPrice(country)
	if err != nil {
		jsonError(w, "failed to get market price: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]any{
		"country":   country,
		"avg_price": price,
	}, http.StatusOK)
}
