package models

// push.go — FCM push notification support.
//
// Architecture:
//   - Device registers its FCM token via POST /api/tma/push-token
//   - Token stored in push_tokens table
//   - SendEpochAlert() is called from the mint queue processor when epoch >= 90%
//   - Actual FCM send requires FCM_SERVER_KEY env var; if missing, just logs
//
// To enable real push:
//   1. Set FCM_SERVER_KEY in .env (from Firebase Console → Project Settings → Cloud Messaging)
//   2. On Android app: request FCM token, POST to /api/tma/push-token on startup

import (
	"bytes"
	"encoding/json"
	"exra/db"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// UpsertPushToken stores or refreshes a device's FCM token.
func UpsertPushToken(deviceID, fcmToken, platform string) error {
	_, err := db.DB.Exec(`
		INSERT INTO push_tokens (device_id, fcm_token, platform, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (device_id, fcm_token) DO UPDATE SET updated_at = NOW()`,
		deviceID, fcmToken, platform,
	)
	return err
}

// fcmPayload is the legacy FCM HTTP v1 send payload.
type fcmPayload struct {
	To           string            `json:"to,omitempty"`
	RegistrationIDs []string       `json:"registration_ids,omitempty"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Icon  string `json:"icon,omitempty"`
}

// SendEpochAlert broadcasts a push notification to all registered devices.
// Called when epoch progress crosses the 90% threshold.
func SendEpochAlert(epochName string, progressPct, daysRemaining float64) {
	serverKey := os.Getenv("FCM_SERVER_KEY")
	if serverKey == "" {
		log.Printf("[Push] FCM_SERVER_KEY not set — skipping epoch alert (epoch=%s progress=%.1f%%)", epochName, progressPct)
		return
	}

	tokens, err := getAllPushTokens(500)
	if err != nil || len(tokens) == 0 {
		log.Printf("[Push] no push tokens registered — skipping epoch alert")
		return
	}

	body := fmt.Sprintf("%.0f%% full — only %.1f days left at current speed!", progressPct, daysRemaining)
	if daysRemaining < 0 {
		body = fmt.Sprintf("%.0f%% full — halving is approaching!", progressPct)
	}

	payload := fcmPayload{
		RegistrationIDs: tokens,
		Notification: fcmNotification{
			Title: fmt.Sprintf("⚡ %s Epoch Alert", epochName),
			Body:  body,
			Icon:  "ic_notification",
		},
		Data: map[string]string{
			"type":           "epoch_alert",
			"epoch_name":     epochName,
			"progress_pct":   fmt.Sprintf("%.2f", progressPct),
			"days_remaining": fmt.Sprintf("%.1f", daysRemaining),
		},
	}

	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(raw))
	if err != nil {
		log.Printf("[Push] failed to build FCM request: %v", err)
		return
	}
	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Push] FCM send error: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[Push] epoch alert sent to %d devices, FCM status=%d", len(tokens), resp.StatusCode)
}

// SendPayoutNotification sends a push to a single device when their payout is processed.
func SendPayoutNotification(deviceID, status string, amountUSD float64) {
	serverKey := os.Getenv("FCM_SERVER_KEY")
	if serverKey == "" {
		return
	}

	tokens, err := getDevicePushTokens(deviceID)
	if err != nil || len(tokens) == 0 {
		return
	}

	title := "💰 Withdrawal Approved"
	body := fmt.Sprintf("$%.2f has been sent to your wallet.", amountUSD)
	if status == "rejected" {
		title = "Withdrawal Rejected"
		body = "Your withdrawal request was rejected. Contact support."
	}

	payload := fcmPayload{
		RegistrationIDs: tokens,
		Notification:    fcmNotification{Title: title, Body: body, Icon: "ic_notification"},
		Data:            map[string]string{"type": "payout_" + status},
	}
	raw, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(raw))
	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
	}
}

func getAllPushTokens(limit int) ([]string, error) {
	rows, err := db.DB.Query(`SELECT fcm_token FROM push_tokens LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func getDevicePushTokens(deviceID string) ([]string, error) {
	rows, err := db.DB.Query(`SELECT fcm_token FROM push_tokens WHERE device_id = $1`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}
