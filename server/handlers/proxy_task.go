package handlers

import (
	"encoding/base64"
	"encoding/json"
	"exra/models"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// POST /api/test/proxy-task
// Sends a proxy task to a connected node and waits for proxy_result.
func DispatchProxyTask(w http.ResponseWriter, r *http.Request) {
	if runtimeHub == nil {
		jsonError(w, "hub is not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		DeviceID       string            `json:"device_id"`
		URL            string            `json:"url"`
		Method         string            `json:"method"`
		Headers        map[string]string `json:"headers"`
		BodyBase64     string            `json:"body_base64"`
		TimeoutSeconds int               `json:"timeout_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.URL == "" {
		jsonError(w, "device_id and url are required", http.StatusBadRequest)
		return
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 20
	}
	if req.Headers == nil {
		req.Headers = map[string]string{}
	}
	if req.BodyBase64 != "" {
		if _, err := base64.StdEncoding.DecodeString(req.BodyBase64); err != nil {
			jsonError(w, "body_base64 must be valid base64", http.StatusBadRequest)
			return
		}
	}

	client, ok := runtimeHub.GetClient(req.DeviceID)
	if !ok {
		jsonError(w, "node is not connected", http.StatusNotFound)
		return
	}

	if models.ShouldInjectCanary(req.DeviceID) {
		task, err := models.CreateCanaryTask(req.DeviceID)
		if err == nil {
			// We send the canary proxy task
			canarySession := "canary-" + uuid.NewString()
			canaryReq := map[string]any{
				"type":        "proxy_task",
				"session_id":  canarySession,
				"method":      "GET",
				"url":         "https://exra.com/canary",
				"headers":     map[string]string{},
				"body_base64": "",
			}
			cPayload, _ := json.Marshal(canaryReq)
			select {
			case client.Send <- cPayload:
				res, ok := runtimeHub.AwaitProxyResult(canarySession, 10*time.Second)
				if !ok {
					models.VerifyCanaryResult(req.DeviceID, task.ID, "timeout")
					jsonError(w, "node canary timeout", http.StatusGatewayTimeout)
					return
				}
				// Verify
				hash := ""
				if bodyDecoded, decodeErr := base64.StdEncoding.DecodeString(res.BodyBase64); decodeErr == nil {
					hash = string(bodyDecoded)
				}
				if !models.VerifyCanaryResult(req.DeviceID, task.ID, hash) {
					jsonError(w, "node failed canary check", http.StatusForbidden)
					return
				}
			default:
				// send queue full, ignore canary for now
			}
		}
	}

	sessionID := uuid.NewString()
	task := map[string]any{
		"type":        "proxy_task",
		"session_id":  sessionID,
		"method":      req.Method,
		"url":         req.URL,
		"headers":     req.Headers,
		"body_base64": req.BodyBase64,
	}
	payload, _ := json.Marshal(task)
	select {
	case client.Send <- payload:
	default:
		jsonError(w, "node send queue is full", http.StatusServiceUnavailable)
		return
	}

	res, ok := runtimeHub.AwaitProxyResult(sessionID, time.Duration(req.TimeoutSeconds)*time.Second)
	if !ok {
		jsonError(w, "timed out waiting for proxy_result", http.StatusGatewayTimeout)
		return
	}
	bodyPreview := ""
	if decoded, err := base64.StdEncoding.DecodeString(res.BodyBase64); err == nil {
		if len(decoded) > 400 {
			bodyPreview = string(decoded[:400])
		} else {
			bodyPreview = string(decoded)
		}
	}

	jsonResponse(w, map[string]any{
		"session_id":   sessionID,
		"device_id":    req.DeviceID,
		"status_code":  res.StatusCode,
		"bytes":        res.Bytes,
		"error":        res.Error,
		"headers":      res.Headers,
		"body_preview": bodyPreview,
		"raw_result":   res,
	}, http.StatusOK)
}

