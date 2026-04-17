package handlers

import (
	"context"
	"encoding/json"
	"exra/db"
	"net/http"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	DB        string `json:"db"`
	Redis     string `json:"redis,omitempty"`
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		DB:        "up",
	}

	// Check Postgres
	if db.DB == nil {
		resp.DB = "down"
		resp.Status = "error"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.DB.PingContext(ctx); err != nil {
			resp.DB = "error: " + err.Error()
			resp.Status = "error"
		}
	}

	// Check Redis via Hub
	if runtimeHub != nil {
		if runtimeHub.IsRedisEnabled() {
			if err := runtimeHub.PingRedis(r.Context()); err != nil {
				resp.Redis = "error: " + err.Error()
				resp.Status = "error"
			} else {
				resp.Redis = "up"
			}
		} else {
			resp.Redis = "disabled (local mode)"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(resp)
}

