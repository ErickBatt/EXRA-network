package main

import (
	"encoding/json"
	"exra/handlers"
	"exra/hub"
	"exra/models"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// StartControlPlane initializes the Gorilla Mux server for discovery and matching.
// Unified with the rest of the project for Phase 2 Cleanup.
func StartControlPlane(port string, h *hub.Hub) {
	r := mux.NewRouter()

	// Discovery (GET /nodes)
	r.HandleFunc("/nodes", func(w http.ResponseWriter, req *http.Request) {
		country := req.URL.Query().Get("country")
		if country == "" {
			country = "ALL"
		}
		tier := req.URL.Query().Get("tier")
		if tier == "" {
			tier = "A"
		}

		nodesRaw, err := h.GetDiscoveryNodes(req.Context(), country, tier, 100)
		if err != nil {
			http.Error(w, "failed to query discovery zset", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[" + joinStrings(nodesRaw, ",") + "]"))
	}).Methods("GET")

	// Market info (GET /nodes/stats)
	r.HandleFunc("/nodes/stats", func(w http.ResponseWriter, req *http.Request) {
		stats, err := models.GetNodeStats()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}).Methods("GET")

	// Matcher Engine
	matcher := &handlers.MatcherHandler{Hub: h}
	r.HandleFunc("/api/offers", matcher.CreateOfferAndMatch).Methods("POST")

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Printf("Control Plane (Gorilla Mux) listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Control Plane failed: %v", err)
		}
	}()

	// Start Task Monitor for Compute Market
	go StartTaskMonitor()
}

// StartTaskMonitor periodically checks for expired compute assignments and penalizes nodes.
func StartTaskMonitor() {
	log.Printf("[Compute] Task Monitor started (Interval: 60s, TTL: 10m)")
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		expiredIDs, err := models.FindExpiredTasks()
		if err != nil {
			log.Printf("[Compute] Task Monitor error querying expired tasks: %v", err)
			continue
		}

		for _, taskID := range expiredIDs {
			log.Printf("[Compute] Task %s expired. Penalizing node and resetting task.", taskID)
			if err := models.FailTask(taskID, "timeout_expired"); err != nil {
				log.Printf("[Compute] Failed to process expiration for task %s: %v", taskID, err)
			}
		}
	}
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	res := s[0]
	for i := 1; i < len(s); i++ {
		res += sep + s[i]
	}
	return res
}
