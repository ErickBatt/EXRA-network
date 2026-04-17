package handlers

import (
	"exra/hub"
	"exra/models"
	"log"
	"net/http"
)

// GET /nodes — returns public node list without sensitive fields (IP, device_id).
func PublicNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := models.GetActiveNodes("")
	if err != nil {
		jsonError(w, "failed to list nodes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pub := make([]models.PublicNode, 0, len(nodes))
	for _, n := range nodes {
		pub = append(pub, models.PublicNode{
			ID:            n.ID,
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

// GET /nodes/stats
func PublicNodeStats(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetNodeStats()
	if err != nil {
		jsonError(w, "failed to get stats: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, stats, http.StatusOK)
}

// GET /ws/map
func LiveMapHandler(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[Map] Upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		id := r.RemoteAddr
		events := h.ListenMapEvents(id)
		defer h.StopMapEvents(id)

		for event := range events {
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}
}
