package handlers

import (
	"encoding/json"
	"net/http"
)

func jsonResponse(w http.ResponseWriter, data interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	jsonResponse(w, map[string]string{"error": msg}, code)
}

