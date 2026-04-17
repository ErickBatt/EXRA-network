package handlers

// tma.go — Telegram Mini App backend endpoints.
//
// All endpoints use NODE_SECRET auth (same as node API) since the TMA runs
// inside the node app and has access to the device_id + node secret.
//
// Routes (registered in main.go under /api/tma/...):
//   GET  /api/tma/me?device_id=xxx         — balance, pending EXRA, gear score, pool info
//   GET  /api/tma/earnings?device_id=xxx   — earnings history (last N events)
//   POST /api/tma/withdraw                 — submit withdrawal request
//   GET  /api/tma/epoch                    — current epoch state with FOMO counter
//   POST /api/tma/push-token               — register FCM push token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"exra/db"
	"exra/models"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// verifyTelegramInitData verifies Telegram WebApp initData HMAC signature.
func verifyTelegramInitData(initData string) (map[string]string, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	params, err := url.ParseQuery(initData)
	if err != nil {
		return nil, err
	}
	hash := params.Get("hash")
	params.Del("hash")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params.Get(k))
	}
	checkString := strings.Join(parts, "\n")

	secretKeyHmac := hmac.New(sha256.New, []byte("WebAppData"))
	secretKeyHmac.Write([]byte(botToken))
	secretKey := secretKeyHmac.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(checkString))
	expected := hex.EncodeToString(mac.Sum(nil))

	if expected != hash {
		return nil, nil // invalid but don't error — return nil so caller can handle
	}
	result := make(map[string]string)
	for k, v := range params {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result, nil
}

// POST /api/tma/auth — authenticate via Telegram initData, return account info
func TmaAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var telegramID int64
	var firstName, username string

	if req.InitData != "" {
		params, err := verifyTelegramInitData(req.InitData)
		if err != nil || params == nil {
			jsonError(w, "invalid telegram auth signature", http.StatusUnauthorized)
			return
		}
		// Parse user from initData
		var tgUser struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		}
		if userJSON := params["user"]; userJSON != "" {
			_ = json.Unmarshal([]byte(userJSON), &tgUser)
			telegramID = tgUser.ID
			firstName = tgUser.FirstName
			username = tgUser.Username
		}
	}

	if telegramID == 0 {
		jsonError(w, "could not identify telegram user", http.StatusBadRequest)
		return
	}

	// Upsert tma_user
	_, err := db.DB.Exec(`
		INSERT INTO tma_users (telegram_id, telegram_username, telegram_first_name, last_seen_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (telegram_id) DO UPDATE
		SET telegram_username = EXCLUDED.telegram_username,
		    telegram_first_name = EXCLUDED.telegram_first_name,
		    last_seen_at = NOW()`,
		telegramID, username, firstName,
	)
	if err != nil {
		log.Printf("tma-auth: upsert tma_user err: %v", err)
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}

	// Get linked devices with their balances (Optimized for 50k nodes)
	// We use the new idx_node_earnings_aggregation to speed up the SUM calculation
	rows, err := db.DB.Query(`
		SELECT td.device_id,
		       COALESCE((
		           SELECT SUM(earned_usd) 
		           FROM node_earnings e 
		           WHERE e.device_id = td.device_id AND e.batch_id IS NULL
		       ), 0) as pending_usd,
		       COALESCE((
		           SELECT SUM(total_credits) 
		           FROM oracle_batches 
		           WHERE oracle_id = td.device_id AND status = 'applied'
		       ), 0) as batched_usd,
		       COALESCE(n.device_type, ''),
		       COALESCE(n.country, ''),
		       COALESCE(n.status, 'offline'),
		       COALESCE(n.identity_tier, 'anon'),
		       COALESCE(n.rs_mult, 0.5)
		FROM tma_devices td
		LEFT JOIN nodes n ON n.device_id = td.device_id
		WHERE td.telegram_id = $1 AND td.status = 'linked'
		ORDER BY td.device_id ASC`, telegramID,
	)
	if err != nil {
		jsonError(w, "failed to load devices", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type DeviceSummary struct {
		DeviceID     string  `json:"device_id"`
		PendingUSD   float64 `json:"pending_usd"`
		BatchedUSD   float64 `json:"batched_usd"`
		DeviceType   string  `json:"device_type"`
		Country      string  `json:"country"`
		Status       string  `json:"status"`
		IdentityTier string  `json:"identity_tier"`
		RSMult       float64 `json:"rs_mult"`
	}

	var devices []DeviceSummary
	var totalUSD float64
	for rows.Next() {
		var d DeviceSummary
		if err := rows.Scan(&d.DeviceID, &d.PendingUSD, &d.BatchedUSD, &d.DeviceType, &d.Country, &d.Status, &d.IdentityTier, &d.RSMult); err != nil {
			log.Printf("tma-auth: scan device err: %v", err)
			continue
		}
		totalUSD += (d.PendingUSD + d.BatchedUSD)
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []DeviceSummary{}
	}

	isPaused := false
	if runtimeHub != nil {
		isPaused = runtimeHub.IsGlobalPause()
	}

	jsonResponse(w, map[string]any{
		"telegram_id":      telegramID,
		"first_name":       firstName,
		"username":         username,
		"devices":          devices,
		"total_earned_usd": totalUSD,
		"global_pause":     isPaused,
	}, http.StatusOK)
}

// POST /api/tma/link-device — link a device_id to Telegram account (approval required)
func TmaLinkDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TelegramID   int64  `json:"telegram_id"`
		DeviceID     string `json:"device_id"`
		TgUser       string `json:"tg_user"`
		TgFirstName  string `json:"tg_first_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TelegramID == 0 || req.DeviceID == "" {
		jsonError(w, "telegram_id and device_id are required", http.StatusBadRequest)
		return
	}
	if err := validateDeviceID(req.DeviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Verify device exists
	var exists bool
	_ = db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM nodes WHERE device_id = $1)`, req.DeviceID).Scan(&exists)
	if !exists {
		jsonError(w, "device not found — make sure the app has connected at least once", http.StatusNotFound)
		return
	}

	// Check if already linked
	var currentStatus string
	_ = db.DB.QueryRow(`SELECT status FROM tma_devices WHERE telegram_id = $1 AND device_id = $2`, req.TelegramID, req.DeviceID).Scan(&currentStatus)
	if currentStatus == "linked" {
		jsonResponse(w, map[string]string{"status": "linked", "message": "device already linked"}, http.StatusOK)
		return
	}

	requestID := models.UUID() // Helper to generate new request ID
	
	_, err := db.DB.Exec(`
		INSERT INTO tma_devices (telegram_id, device_id, status, request_id)
		VALUES ($1, $2, 'pending', $3)
		ON CONFLICT (telegram_id, device_id) DO UPDATE
		SET status = 'pending', request_id = EXCLUDED.request_id`,
		req.TelegramID, req.DeviceID, requestID,
	)
	if err != nil {
		jsonError(w, "failed to initiate link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Send WS request to node
	if runtimeHub != nil {
		runtimeHub.BroadcastLinkRequest(req.DeviceID, req.TgUser, req.TgFirstName, requestID)
	}

	jsonResponse(w, map[string]string{
		"status": "pending", 
		"request_id": requestID,
		"message": "Please approve the request on your mobile device",
	}, http.StatusAccepted)
}

// GET /api/tma/me?device_id=xxx
func TmaMe(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	if err := validateDeviceID(deviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Pending credits (unbatched)
	var pendingUSD float64
	_ = db.DB.QueryRow(`
		SELECT COALESCE(SUM(earned_usd), 0)
		FROM node_earnings
		WHERE device_id = $1 AND batch_id IS NULL`,
		deviceID,
	).Scan(&pendingUSD)

	// Batched credits (consensus reached)
	var batchedCredits float64
	_ = db.DB.QueryRow(`
		SELECT COALESCE(SUM(total_credits), 0)
		FROM oracle_batches
		WHERE oracle_id = $1 AND status IN ('consensus', 'applied')`,
		deviceID,
	).Scan(&batchedCredits)

	// Gear score from node hardware
	var cpuCores, vramMB, ramMB, bandwidthMbps int
	var deviceType string
	var isResidential bool
	_ = db.DB.QueryRow(`
		SELECT COALESCE(cpu_cores,0), COALESCE(vram_mb,0), COALESCE(ram_mb,0),
		       COALESCE(bandwidth_mbps,0), COALESCE(device_type,''), COALESCE(is_residential,false)
		FROM nodes WHERE device_id = $1`, deviceID,
	).Scan(&cpuCores, &vramMB, &ramMB, &bandwidthMbps, &deviceType, &isResidential)
	gs := models.ComputeGearScore(deviceType, cpuCores, vramMB, ramMB, bandwidthMbps, isResidential)

	// Pool membership
	pool, _ := models.GetPoolByDevice(nil, deviceID)
	poolInfo := map[string]any{"pool": nil, "tier": "solo", "treasury_fee_pct": 30}
	if pool != nil {
		poolInfo = map[string]any{
			"pool":             pool,
			"tier":             pool.Tier,
			"treasury_fee_pct": pool.TreasuryFeePct,
		}
	}

	// Identity Tier info
	var identityTier, did string
	var rsMult float64
	_ = db.DB.QueryRow(`SELECT identity_tier, rs_mult, did FROM nodes WHERE device_id = $1`, deviceID).Scan(&identityTier, &rsMult, &did)

	taxRate := 25
	if identityTier == "peak" {
		taxRate = 0
	}

	isPaused := false
	if runtimeHub != nil {
		isPaused = runtimeHub.IsGlobalPause()
	}

	jsonResponse(w, map[string]any{
		"device_id":       deviceID,
		"did":             did,
		"pending_usd":     pendingUSD,
		"batched_credits": batchedCredits,
		"identity_tier":   identityTier,
		"rs_multiplier":   rsMult,
		"gear_score":      gs,
		"pool":            poolInfo,
		"tax_rate_pct":    taxRate,
		"global_pause":    isPaused,
	}, http.StatusOK)
}

// POST /api/tma/stake — upgrade to Peak tier using 100 EXRA from batched_credits
func TmaStake(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Get DID and current balance
	var did, tier string
	var batchedCredits float64
	err := db.DB.QueryRow(`
		SELECT n.did, n.identity_tier, COALESCE(SUM(b.total_credits), 0)
		FROM nodes n
		LEFT JOIN oracle_batches b ON b.oracle_id = n.device_id AND b.status = 'applied'
		WHERE n.device_id = $1
		GROUP BY n.did, n.identity_tier`, req.DeviceID).Scan(&did, &tier, &batchedCredits)

	if err != nil || did == "" {
		jsonError(w, "node not found or missing DID", http.StatusNotFound)
		return
	}

	if tier == "peak" {
		jsonError(w, "already at Peak tier", http.StatusBadRequest)
		return
	}

	if batchedCredits < 100 {
		jsonError(w, fmt.Sprintf("insufficient credits: 100 EXRA required, you have %.2f", batchedCredits), http.StatusPaymentRequired)
		return
	}

	// 2. Perform upgrade via models
	if err := models.UpgradeNodeToPeak(did, 100); err != nil {
		jsonError(w, "staking failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "success",
		"message": "Node upgraded to Peak tier! 100 EXRA staked.",
	}, http.StatusOK)
}

// GET /api/tma/earnings?device_id=xxx&limit=50
func TmaEarnings(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	limit := 50
	rows, err := db.DB.Query(`
		SELECT id, total_emission, worker_reward, referral_reward, treasury_reward,
		       reason_code, policy_snapshot, created_at
		FROM pop_reward_events
		WHERE device_id = $1
		ORDER BY created_at DESC
		LIMIT $2`,
		deviceID, limit,
	)
	if err != nil {
		jsonError(w, "failed to load earnings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type EarningRow struct {
		ID             int64           `json:"id"`
		TotalEmission  float64         `json:"total_emission"`
		WorkerReward   float64         `json:"worker_reward"`
		ReferralReward float64         `json:"referral_reward"`
		TreasuryReward float64         `json:"treasury_reward"`
		ReasonCode     string          `json:"reason_code"`
		Snapshot       json.RawMessage `json:"snapshot"`
		CreatedAt      string          `json:"created_at"`
	}

	var out []EarningRow
	for rows.Next() {
		var row EarningRow
		var snap []byte
		if err := rows.Scan(&row.ID, &row.TotalEmission, &row.WorkerReward,
			&row.ReferralReward, &row.TreasuryReward, &row.ReasonCode, &snap, &row.CreatedAt); err != nil {
			continue
		}
		row.Snapshot = json.RawMessage(snap)
		out = append(out, row)
	}
	if out == nil {
		out = []EarningRow{}
	}
	jsonResponse(w, map[string]any{"device_id": deviceID, "events": out}, http.StatusOK)
}

// POST /api/tma/withdraw
func TmaWithdraw(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID        string  `json:"device_id"`
		AmountUSD       float64 `json:"amount_usd"`
		RecipientWallet string  `json:"recipient_wallet"`
		Currency        string  `json:"currency"` 
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.AmountUSD <= 0 || req.RecipientWallet == "" {
		jsonError(w, "device_id, amount_usd, and recipient_wallet are required", http.StatusBadRequest)
		return
	}
	if err := validateDeviceID(req.DeviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. Fetch DID for this device (Withdrawals require on-chain identity in v2.0)
	var did string
	err := db.DB.QueryRow(`SELECT did FROM nodes WHERE device_id = $1`, req.DeviceID).Scan(&did)
	if err != nil || did == "" {
		jsonError(w, "withdrawal failed: device has no associated PEAQ DID", http.StatusForbidden)
		return
	}

	// 2. Perform Claim via PEAQ v2.0 Logic (Tax, Timelock, Velocity)
	payout, err := models.ClaimPayout(did, req.AmountUSD, req.RecipientWallet)
	if err != nil {
		log.Printf("tma-withdraw: ClaimPayout err: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Get Velocity info for the response (timelock count down)
	var taxAmount float64
	var eligibleAt time.Time
	var tier string
	_ = db.DB.QueryRow(`
		SELECT tax_amount, eligible_at, tier_at_payout 
		FROM did_payout_velocity 
		WHERE payout_id = $1`, payout.ID).Scan(&taxAmount, &eligibleAt, &tier)

	jsonResponse(w, map[string]any{
		"status":      "pending",
		"payout_id":   payout.ID,
		"amount_usd":  payout.AmountUSD,
		"tax_amount":  taxAmount,
		"net_amount":  payout.NetAmountUSD,
		"eligible_at": eligibleAt,
		"tier":        tier,
		"message":     "Withdrawal initiated. Subject to " + tier + " timelock.",
	}, http.StatusCreated)
}

func TmaEpoch(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetTokenomicsStats()
	if err != nil {
		jsonError(w, "failed to load stats", http.StatusInternalServerError)
		return
	}
	dailyRate := models.GetAvgDailyMintRate()
	epoch := models.CheckCurrentEpochWithRate(stats.TotalExraMinted, dailyRate)
	jsonResponse(w, epoch, http.StatusOK)
}

// POST /api/tma/push-token — register FCM token for push notifications
func TmaRegisterPushToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		FCMToken string `json:"fcm_token"`
		Platform string `json:"platform"` // "android" | "ios"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceID == "" || req.FCMToken == "" {
		jsonError(w, "device_id and fcm_token are required", http.StatusBadRequest)
		return
	}
	if req.Platform == "" {
		req.Platform = "android"
	}
	if req.Platform != "android" && req.Platform != "ios" {
		jsonError(w, "platform must be android or ios", http.StatusBadRequest)
		return
	}
	if err := models.UpsertPushToken(req.DeviceID, req.FCMToken, req.Platform); err != nil {
		jsonError(w, "failed to register push token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "registered"}, http.StatusOK)
}
