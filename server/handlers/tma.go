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
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// ── G2 Sybil helpers ────────────────────────────────────────────────────────

// extractClientIP returns the real client IP, preferring reverse-proxy headers.
func extractClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// maskSubnet reduces an IP to its /24 (IPv4) or /48 (IPv6) prefix.
// Returns "" for invalid IPs so callers can skip the check gracefully.
func maskSubnet(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	if ip4 := ip.To4(); ip4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", ip4[0], ip4[1], ip4[2])
	}
	ip6 := ip.To16()
	return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x::/48",
		ip6[0], ip6[1], ip6[2], ip6[3], ip6[4], ip6[5])
}

// ── Auth helpers ─────────────────────────────────────────────────────────────

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

	// Sybil limit 1: max 5 linked devices per Telegram account.
	var linkedCount int
	if err := db.DB.QueryRow(
		`SELECT COUNT(*) FROM tma_devices WHERE telegram_id=$1 AND status='linked'`, p.TelegramID,
	).Scan(&linkedCount); err == nil && linkedCount >= 5 {
		jsonError(w, "max 5 devices per Telegram account", http.StatusForbidden)
		return
	}

	// Sybil limit 2 (G2): max 3 linked devices per /24 (IPv4) or /48 (IPv6) subnet.
	// Prevents farm operators from registering hundreds of VMs from the same DC.
	clientIP := extractClientIP(r)
	subnet := maskSubnet(clientIP)
	if subnet != "" {
		var subnetCount int
		_ = db.DB.QueryRow(
			`SELECT COUNT(*) FROM tma_devices WHERE ip_subnet=$1 AND status='linked'`, subnet,
		).Scan(&subnetCount)
		if subnetCount >= 3 {
			log.Printf("tma-link: G2 subnet block ip=%s subnet=%s tg=%d", clientIP, subnet, p.TelegramID)
			jsonError(w, "too many devices registered from this network — contact support if incorrect",
				http.StatusTooManyRequests)
			return
		}
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
		INSERT INTO tma_devices (telegram_id, device_id, status, request_id, linked_ip, ip_subnet)
		VALUES ($1, $2, 'pending', $3, $4, $5)
		ON CONFLICT (telegram_id, device_id) DO UPDATE
		SET status = 'pending', request_id = EXCLUDED.request_id,
		    linked_ip = EXCLUDED.linked_ip, ip_subnet = EXCLUDED.ip_subnet`,
		p.TelegramID, req.DeviceID, requestID, clientIP, subnet,
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

// ── Worker Marketplace Listings ─────────────────────────────────────────────
//
// Workers list capacity from TMA; buyers browse via /api/marketplace/lots
// (public, no auth). The two surfaces use different auth — dual-auth by design.

// POST /api/tma/lots/create — create or reprice a marketplace listing.
// Guards: TMAAuth cookie + device ownership + ≥3 PoP sessions (Sybil gate).
func TmaCreateLot(w http.ResponseWriter, r *http.Request) {
	tgID := middleware.TelegramIDFromContext(r)

	var req struct {
		DeviceID      string  `json:"device_id"`
		PricePerGB    float64 `json:"price_per_gb"`
		BandwidthMbps int     `json:"bandwidth_mbps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.PricePerGB <= 0 || req.PricePerGB >= 1000 {
		jsonError(w, "price_per_gb must be between 0 and 1000", http.StatusBadRequest)
		return
	}
	if req.BandwidthMbps <= 0 {
		jsonError(w, "bandwidth_mbps must be positive", http.StatusBadRequest)
		return
	}

	deviceID := requireDeviceOwnership(w, r, req.DeviceID)
	if deviceID == "" {
		return
	}

	// Frozen nodes cannot list — prevents rewarding bad actors.
	var nodeStatus string
	_ = db.DB.QueryRow(`SELECT COALESCE(status,'offline') FROM nodes WHERE device_id=$1`, deviceID).Scan(&nodeStatus)
	if nodeStatus == "frozen" {
		jsonError(w, "frozen devices cannot create listings", http.StatusForbidden)
		return
	}

	// Sybil gate: require proven work before a listing goes live.
	var popSessions int
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM pop_reward_events WHERE device_id=$1`, deviceID).Scan(&popSessions)
	if popSessions < 3 {
		jsonError(w, "device needs ≥3 completed PoP sessions before listing — earn trust first",
			http.StatusForbidden)
		return
	}

	var gearScore float64
	var identityTier string
	_ = db.DB.QueryRow(
		`SELECT COALESCE(rs_mult,0.5), COALESCE(identity_tier,'anon') FROM nodes WHERE device_id=$1`,
		deviceID,
	).Scan(&gearScore, &identityTier)

	var lotID string
	err := db.DB.QueryRow(`
		INSERT INTO worker_listings
		    (telegram_id, device_id, price_per_gb, bandwidth_mbps,
		     gear_score, identity_tier, pop_sessions, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'active')
		ON CONFLICT (device_id) DO UPDATE SET
		    telegram_id    = EXCLUDED.telegram_id,
		    price_per_gb   = EXCLUDED.price_per_gb,
		    bandwidth_mbps = EXCLUDED.bandwidth_mbps,
		    gear_score     = EXCLUDED.gear_score,
		    identity_tier  = EXCLUDED.identity_tier,
		    pop_sessions   = EXCLUDED.pop_sessions,
		    status         = 'active'
		RETURNING id`,
		tgID, deviceID, req.PricePerGB, req.BandwidthMbps, gearScore, identityTier, popSessions,
	).Scan(&lotID)
	if err != nil {
		log.Printf("tma-lots: create tg=%d dev=%s: %v", tgID, deviceID, err)
		jsonError(w, "failed to create listing", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]any{
		"lot_id":        lotID,
		"device_id":     deviceID,
		"status":        "active",
		"price_per_gb":  req.PricePerGB,
		"gear_score":    gearScore,
		"identity_tier": identityTier,
		"pop_sessions":  popSessions,
	}, http.StatusCreated)
}

// GET /api/tma/lots — worker's own listings (cookie-session).
func TmaListMyLots(w http.ResponseWriter, r *http.Request) {
	tgID := middleware.TelegramIDFromContext(r)

	rows, err := db.DB.Query(`
		SELECT wl.id, wl.device_id, wl.price_per_gb, wl.bandwidth_mbps,
		       wl.gear_score, wl.identity_tier, wl.status, wl.pop_sessions, wl.updated_at,
		       COALESCE(n.status,'offline') as node_status
		FROM worker_listings wl
		LEFT JOIN nodes n ON n.device_id = wl.device_id
		WHERE wl.telegram_id = $1
		ORDER BY wl.updated_at DESC`, tgID)
	if err != nil {
		jsonError(w, "failed to load listings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ListingRow struct {
		ID            string  `json:"id"`
		DeviceID      string  `json:"device_id"`
		PricePerGB    float64 `json:"price_per_gb"`
		BandwidthMbps int     `json:"bandwidth_mbps"`
		GearScore     float64 `json:"gear_score"`
		IdentityTier  string  `json:"identity_tier"`
		Status        string  `json:"status"`
		PopSessions   int     `json:"pop_sessions"`
		UpdatedAt     string  `json:"updated_at"`
		NodeStatus    string  `json:"node_status"`
	}

	var out []ListingRow
	for rows.Next() {
		var row ListingRow
		if err := rows.Scan(
			&row.ID, &row.DeviceID, &row.PricePerGB, &row.BandwidthMbps,
			&row.GearScore, &row.IdentityTier, &row.Status,
			&row.PopSessions, &row.UpdatedAt, &row.NodeStatus,
		); err != nil {
			log.Printf("tma-lots: scan err tg=%d: %v", tgID, err)
			continue
		}
		out = append(out, row)
	}
	if out == nil {
		out = []ListingRow{}
	}
	jsonResponse(w, map[string]any{"listings": out}, http.StatusOK)
}

// POST /api/tma/lots/{id}/pause — pause an active listing.
func TmaPauseLot(w http.ResponseWriter, r *http.Request) {
	tgID := middleware.TelegramIDFromContext(r)
	lotID := mux.Vars(r)["id"]
	if lotID == "" {
		jsonError(w, "lot id required", http.StatusBadRequest)
		return
	}
	res, err := db.DB.Exec(
		`UPDATE worker_listings SET status='paused'
		 WHERE id=$1 AND telegram_id=$2 AND status='active'`,
		lotID, tgID,
	)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonError(w, "listing not found or not active", http.StatusNotFound)
		return
	}
	jsonResponse(w, map[string]string{"status": "paused"}, http.StatusOK)
}

// POST /api/tma/lots/{id}/resume — resume a paused listing.
func TmaResumeLot(w http.ResponseWriter, r *http.Request) {
	tgID := middleware.TelegramIDFromContext(r)
	lotID := mux.Vars(r)["id"]
	if lotID == "" {
		jsonError(w, "lot id required", http.StatusBadRequest)
		return
	}
	res, err := db.DB.Exec(
		`UPDATE worker_listings SET status='active'
		 WHERE id=$1 AND telegram_id=$2 AND status='paused'`,
		lotID, tgID,
	)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonError(w, "listing not found or not paused", http.StatusNotFound)
		return
	}
	jsonResponse(w, map[string]string{"status": "active"}, http.StatusOK)
}

// DELETE /api/tma/lots/{id} — soft-delete a listing (status='deleted', never purged).
// Immutable audit trail: deleted listings remain in DB for fraud investigation.
func TmaDeleteLot(w http.ResponseWriter, r *http.Request) {
	tgID := middleware.TelegramIDFromContext(r)
	lotID := mux.Vars(r)["id"]
	if lotID == "" {
		jsonError(w, "lot id required", http.StatusBadRequest)
		return
	}
	res, err := db.DB.Exec(
		`UPDATE worker_listings SET status='deleted'
		 WHERE id=$1 AND telegram_id=$2 AND status IN ('active','paused')`,
		lotID, tgID,
	)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonError(w, "listing not found or already deleted", http.StatusNotFound)
		return
	}
	jsonResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)
}
