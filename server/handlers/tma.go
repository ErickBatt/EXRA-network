package handlers

// tma.go — Telegram Mini App backend endpoints.
//
// Authentication model (v2.4.1 hardened):
//   * POST /api/tma/auth         — public, requires valid initData (HMAC+auth_date TTL).
//                                  Issues HttpOnly cookie session (TMA JWT, 24h).
//   * POST /api/tma/link-device  — requires valid initData in body (telegram_id derived
//                                  from signed params only, never from client body).
//   * All other /api/tma/* routes require the TMAAuth cookie session + ownership check
//     in tma_devices (telegram_id ↔ device_id, status='linked').
//
// Replaces the prior model which trusted a shared NODE_SECRET + client-supplied
// telegram_id, which allowed cross-account balance reads and withdrawals.

import (
	"encoding/json"
	"errors"
	"exra/db"
	"exra/middleware"
	"exra/models"
	"fmt"
	"log"
	"net/http"
	"time"
)

func verifyInitDataOrReject(w http.ResponseWriter, initData string) *middleware.TMAInitDataParams {
	if initData == "" {
		jsonError(w, "init_data is required", http.StatusBadRequest)
		return nil
	}
	p, err := middleware.VerifyTelegramInitData(initData)
	if err != nil {
		switch {
		case errors.Is(err, middleware.ErrTMAExpiredInitData):
			jsonError(w, "telegram initData expired — reopen app", http.StatusUnauthorized)
		case errors.Is(err, middleware.ErrTMAInvalidSignature):
			jsonError(w, "invalid telegram signature", http.StatusUnauthorized)
		default:
			jsonError(w, "invalid telegram initData", http.StatusUnauthorized)
		}
		return nil
	}
	return p
}

// POST /api/tma/auth — verify initData, upsert tma_users, issue cookie, return account.
func TmaAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	p := verifyInitDataOrReject(w, req.InitData)
	if p == nil {
		return
	}

	if _, err := db.DB.Exec(`
		INSERT INTO tma_users (telegram_id, telegram_username, telegram_first_name, last_seen_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (telegram_id) DO UPDATE
		SET telegram_username = EXCLUDED.telegram_username,
		    telegram_first_name = EXCLUDED.telegram_first_name,
		    last_seen_at = NOW()`,
		p.TelegramID, p.Username, p.FirstName,
	); err != nil {
		log.Printf("tma-auth: upsert err: %v", err)
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}

	if err := middleware.IssueTMASession(w, p); err != nil {
		log.Printf("tma-auth: issue session err: %v", err)
		jsonError(w, "session error", http.StatusInternalServerError)
		return
	}

	writeAccountSummary(w, p.TelegramID, p.FirstName, p.Username)
}

// writeAccountSummary loads linked devices + totals and writes the JSON response.
func writeAccountSummary(w http.ResponseWriter, telegramID int64, firstName, username string) {
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
	var totalPending, totalBatched float64
	for rows.Next() {
		var d DeviceSummary
		if err := rows.Scan(&d.DeviceID, &d.PendingUSD, &d.BatchedUSD, &d.DeviceType, &d.Country, &d.Status, &d.IdentityTier, &d.RSMult); err != nil {
			log.Printf("tma-auth: scan device err: %v", err)
			continue
		}
		totalPending += d.PendingUSD
		totalBatched += d.BatchedUSD
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
		"telegram_id":        telegramID,
		"first_name":         firstName,
		"username":           username,
		"devices":            devices,
		"pending_usd":        totalPending,      // informational: not yet withdrawable
		"withdrawable_usd":   totalBatched,      // applied batches only
		"total_earned_usd":   totalPending + totalBatched,
		"global_pause":       isPaused,
	}, http.StatusOK)
}

// GET /api/tma/me — returns account summary for the currently authenticated session.
// Cookie-session protected.
func TmaMeSession(w http.ResponseWriter, r *http.Request) {
	tgID := middleware.TelegramIDFromContext(r)
	firstName := middleware.TelegramFirstNameFromContext(r)
	username := middleware.TelegramUsernameFromContext(r)
	writeAccountSummary(w, tgID, firstName, username)
}

// POST /api/tma/link-device — initiate device link. Requires signed initData.
// `telegram_id`, `tg_user`, `tg_first_name` are derived from initData only.
func TmaLinkDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InitData string `json:"init_data"`
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	p := verifyInitDataOrReject(w, req.InitData)
	if p == nil {
		return
	}
	if req.DeviceID == "" {
		jsonError(w, "device_id is required", http.StatusBadRequest)
		return
	}
	if err := validateDeviceID(req.DeviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Sybil limit: max 5 linked devices per Telegram account.
	var linkedCount int
	if err := db.DB.QueryRow(
		`SELECT COUNT(*) FROM tma_devices WHERE telegram_id=$1 AND status='linked'`, p.TelegramID,
	).Scan(&linkedCount); err == nil && linkedCount >= 5 {
		jsonError(w, "max 5 devices per Telegram account", http.StatusForbidden)
		return
	}

	var exists bool
	_ = db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM nodes WHERE device_id = $1)`, req.DeviceID).Scan(&exists)
	if !exists {
		jsonError(w, "device not found — make sure the app has connected at least once", http.StatusNotFound)
		return
	}

	var currentStatus string
	_ = db.DB.QueryRow(`SELECT status FROM tma_devices WHERE telegram_id=$1 AND device_id=$2`,
		p.TelegramID, req.DeviceID).Scan(&currentStatus)
	if currentStatus == "linked" {
		jsonResponse(w, map[string]string{"status": "linked", "message": "device already linked"}, http.StatusOK)
		return
	}

	requestID := models.UUID()
	_, err := db.DB.Exec(`
		INSERT INTO tma_devices (telegram_id, device_id, status, request_id)
		VALUES ($1, $2, 'pending', $3)
		ON CONFLICT (telegram_id, device_id) DO UPDATE
		SET status = 'pending', request_id = EXCLUDED.request_id`,
		p.TelegramID, req.DeviceID, requestID,
	)
	if err != nil {
		jsonError(w, "failed to initiate link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if runtimeHub != nil {
		runtimeHub.BroadcastLinkRequest(req.DeviceID, p.Username, p.FirstName, requestID)
	}

	jsonResponse(w, map[string]string{
		"status":     "pending",
		"request_id": requestID,
		"message":    "Please approve the request on your mobile device",
	}, http.StatusAccepted)
}

// requireDeviceOwnership fetches device_id from query/body and ensures the caller owns it.
// Returns the verified device_id or empty string if response has already been written.
func requireDeviceOwnership(w http.ResponseWriter, r *http.Request, deviceID string) string {
	if err := validateDeviceID(deviceID); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return ""
	}
	tgID := middleware.TelegramIDFromContext(r)
	owned, err := middleware.AssertDeviceOwnedByTelegram(tgID, deviceID)
	if err != nil {
		jsonError(w, "ownership check failed", http.StatusInternalServerError)
		return ""
	}
	if !owned {
		jsonError(w, "device not linked to this Telegram account", http.StatusForbidden)
		return ""
	}
	return deviceID
}

// GET /api/tma/device?device_id=xxx — per-device detail (cookie-session + ownership).
func TmaMe(w http.ResponseWriter, r *http.Request) {
	deviceID := requireDeviceOwnership(w, r, r.URL.Query().Get("device_id"))
	if deviceID == "" {
		return
	}

	var pendingUSD float64
	_ = db.DB.QueryRow(`
		SELECT COALESCE(SUM(earned_usd), 0)
		FROM node_earnings
		WHERE device_id = $1 AND batch_id IS NULL`,
		deviceID,
	).Scan(&pendingUSD)

	var batchedCredits float64
	_ = db.DB.QueryRow(`
		SELECT COALESCE(SUM(total_credits), 0)
		FROM oracle_batches
		WHERE oracle_id = $1 AND status IN ('consensus', 'applied')`,
		deviceID,
	).Scan(&batchedCredits)

	var cpuCores, vramMB, ramMB, bandwidthMbps int
	var deviceType string
	var isResidential bool
	_ = db.DB.QueryRow(`
		SELECT COALESCE(cpu_cores,0), COALESCE(vram_mb,0), COALESCE(ram_mb,0),
		       COALESCE(bandwidth_mbps,0), COALESCE(device_type,''), COALESCE(is_residential,false)
		FROM nodes WHERE device_id = $1`, deviceID,
	).Scan(&cpuCores, &vramMB, &ramMB, &bandwidthMbps, &deviceType, &isResidential)
	gs := models.ComputeGearScore(deviceType, cpuCores, vramMB, ramMB, bandwidthMbps, isResidential)

	pool, _ := models.GetPoolByDevice(nil, deviceID)
	poolInfo := map[string]any{"pool": nil, "tier": "solo", "treasury_fee_pct": 30}
	if pool != nil {
		poolInfo = map[string]any{
			"pool":             pool,
			"tier":             pool.Tier,
			"treasury_fee_pct": pool.TreasuryFeePct,
		}
	}

	var identityTier, did string
	var rsMult float64
	_ = db.DB.QueryRow(`SELECT identity_tier, rs_mult, did FROM nodes WHERE device_id = $1`, deviceID).Scan(&identityTier, &rsMult, &did)

	taxRate := 25
	if identityTier == "peak" {
		taxRate = 0
	}

	// Next-eligible payout from timelock.
	var nextEligibleAt *time.Time
	var et time.Time
	if err := db.DB.QueryRow(`
		SELECT MIN(eligible_at)
		FROM did_payout_velocity
		WHERE did = $1 AND eligible_at > NOW()`, did).Scan(&et); err == nil && !et.IsZero() {
		nextEligibleAt = &et
	}

	isPaused := false
	if runtimeHub != nil {
		isPaused = runtimeHub.IsGlobalPause()
	}

	jsonResponse(w, map[string]any{
		"device_id":        deviceID,
		"did":              did,
		"pending_usd":      pendingUSD,
		"batched_credits":  batchedCredits,
		"identity_tier":    identityTier,
		"rs_multiplier":    rsMult,
		"gear_score":       gs,
		"pool":             poolInfo,
		"tax_rate_pct":     taxRate,
		"next_eligible_at": nextEligibleAt,
		"global_pause":     isPaused,
	}, http.StatusOK)
}

// POST /api/tma/stake — upgrade to Peak tier. Cookie-session + ownership required.
func TmaStake(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	deviceID := requireDeviceOwnership(w, r, req.DeviceID)
	if deviceID == "" {
		return
	}

	var did, tier string
	var batchedCredits float64
	err := db.DB.QueryRow(`
		SELECT n.did, n.identity_tier, COALESCE(SUM(b.total_credits), 0)
		FROM nodes n
		LEFT JOIN oracle_batches b ON b.oracle_id = n.device_id AND b.status = 'applied'
		WHERE n.device_id = $1
		GROUP BY n.did, n.identity_tier`, deviceID).Scan(&did, &tier, &batchedCredits)

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
	if err := models.UpgradeNodeToPeak(did, 100); err != nil {
		jsonError(w, "staking failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{
		"status":  "success",
		"message": "Node upgraded to Peak tier! 100 EXRA staked.",
	}, http.StatusOK)
}

// GET /api/tma/earnings?device_id=xxx — cookie-session + ownership required.
func TmaEarnings(w http.ResponseWriter, r *http.Request) {
	deviceID := requireDeviceOwnership(w, r, r.URL.Query().Get("device_id"))
	if deviceID == "" {
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

// POST /api/tma/withdraw — cookie-session + ownership required.
func TmaWithdraw(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID        string  `json:"device_id"`
		AmountUSD       float64 `json:"amount_usd"`
		RecipientWallet string  `json:"recipient_wallet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.AmountUSD <= 0 || req.RecipientWallet == "" {
		jsonError(w, "amount_usd and recipient_wallet are required", http.StatusBadRequest)
		return
	}
	deviceID := requireDeviceOwnership(w, r, req.DeviceID)
	if deviceID == "" {
		return
	}

	var did string
	err := db.DB.QueryRow(`SELECT did FROM nodes WHERE device_id = $1`, deviceID).Scan(&did)
	if err != nil || did == "" {
		jsonError(w, "withdrawal failed: device has no associated PEAQ DID", http.StatusForbidden)
		return
	}

	payout, err := models.ClaimPayout(did, req.AmountUSD, req.RecipientWallet)
	if err != nil {
		log.Printf("tma-withdraw: ClaimPayout err: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

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

// GET /api/tma/epoch — public.
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

// POST /api/tma/push-token — cookie-session + ownership required.
func TmaRegisterPushToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		FCMToken string `json:"fcm_token"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.FCMToken == "" {
		jsonError(w, "fcm_token is required", http.StatusBadRequest)
		return
	}
	deviceID := requireDeviceOwnership(w, r, req.DeviceID)
	if deviceID == "" {
		return
	}
	if req.Platform == "" {
		req.Platform = "android"
	}
	if req.Platform != "android" && req.Platform != "ios" {
		jsonError(w, "platform must be android or ios", http.StatusBadRequest)
		return
	}
	if err := models.UpsertPushToken(deviceID, req.FCMToken, req.Platform); err != nil {
		jsonError(w, "failed to register push token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, map[string]string{"status": "registered"}, http.StatusOK)
}

// POST /api/tma/logout — clear cookie.
func TmaLogout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearTMASession(w)
	jsonResponse(w, map[string]string{"status": "logged_out"}, http.StatusOK)
}
