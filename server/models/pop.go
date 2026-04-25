package models

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"exra/db"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"context"
)

// HubProvider decouples models from the hub package to prevent import cycles.
type HubProvider interface {
	BroadcastFeederTask(feederID string, assignmentID int64, targetDeviceID, targetIP string, targetPort int)
	SyncNodeToRedis(ctx context.Context, country, rsTier string, score float64, pubNodeJSON string) error
	SetGlobalPause(active bool)
	IsGlobalPause() bool
}

var (
	globalHub HubProvider
)

func SetHub(h HubProvider) {
	globalHub = h
}

// DefaultPopEmission is the fallback Exra emission per heartbeat tick.
// Override via POP_EMISSION_PER_HEARTBEAT env → models.SetPopEmission.
var defaultPopEmission float64 = 0.000050

// Halving and epoch multiplier are based on total minted supply (CalculateEpochMultiplier).

// supplyCache guards totalMintedCached and lastSupplyUpdate against
// concurrent access from multiple HeartbeatPoP goroutines.
var supplyMu          sync.Mutex
var totalMintedCached float64
var lastSupplyUpdate  time.Time

// Scaling: Buffer heartbeats to avoid DB contention at 50k nodes
var popChannel = make(chan popRequest, 32768)

type popRequest struct {
	DeviceID string
	Emission float64
}

// SetPopEmission allows main.go to inject the env-configured value.
func SetPopEmission(e float64) {
	if e > 0 {
		defaultPopEmission = e
	}
}

// GetPopEmission returns the current PoP emission per heartbeat.
func GetPopEmission() float64 {
	return defaultPopEmission
}

// ---- Referral tier logic ------------------------------------------------

// ReferralTier describes a referral level with name and percentage.
type ReferralTier struct {
	Level   int
	Name    string
	MinRefs int
	MaxRefs int
	Percent float64 // 0.10 … 0.30
}

// referralTiers defines the 4-level inclusive scale (boundary values inclusive).
var referralTiers = []ReferralTier{
	{Level: 4, Name: "Ambassador", MinRefs: 601, MaxRefs: 1000000, Percent: 0.30},
	{Level: 3, Name: "Crypto Boss", MinRefs: 301, MaxRefs: 600, Percent: 0.20},
	{Level: 2, Name: "Network Builder", MinRefs: 101, MaxRefs: 300, Percent: 0.15},
	{Level: 1, Name: "Street Scout", MinRefs: 1, MaxRefs: 100, Percent: 0.10},
}

// ReferralPercent returns the referral reward percentage for a given referral
// count. Returns 0 if the referrer has no referrals yet.
func ReferralPercent(referralCount int) float64 {
	if referralCount > 1000000 {
		return 0.30 // Ambassador cap
	}
	for _, t := range referralTiers {
		if referralCount >= t.MinRefs && referralCount <= t.MaxRefs {
			return t.Percent
		}
	}
	return 0
}

// ReferralTierByCount returns the full ReferralTier struct for a count (nil if none).
func ReferralTierByCount(referralCount int) *ReferralTier {
	if referralCount > 1000000 {
		return &referralTiers[0] // Ambassador
	}
	for i, t := range referralTiers {
		if referralCount >= t.MinRefs && referralCount <= t.MaxRefs {
			return &referralTiers[i]
		}
	}
	return nil
}

// ---- PoP distribution ------------------------------------------------

// PopRewardSnapshot is stored in policy_snapshot JSONB for audit.
type PopRewardSnapshot struct {
	TotalEmission    float64 `json:"total_emission"`
	WorkerReward     float64 `json:"worker_reward"`
	ReferralPercent  float64 `json:"referral_percent"`
	ReferralReward   float64 `json:"referral_reward"`
	ReferrerDeviceID string  `json:"referrer_device_id,omitempty"`
	TreasuryReward   float64 `json:"treasury_reward"`
	ReferralCount    int     `json:"referral_count"`
	TierName         string  `json:"tier_name,omitempty"`
	ReasonCode       string  `json:"reason_code"`
	// Epoch & Gear Score (supply-based tokenomics)
	EpochIndex      int     `json:"epoch_index"`
	EpochMultiplier float64 `json:"epoch_multiplier"`
	GearGrade       string  `json:"gear_grade"`
	GearScore       int     `json:"gear_score"`
	GearMultiplier  float64 `json:"gear_multiplier"`
	TotalMultiplier float64 `json:"total_multiplier"`
}

// idempotencyKey generates a unique key per device per 30-second bucket.
// A second call within the same window returns the same key → INSERT will
// hit the UNIQUE constraint and be silently skipped.
func idempotencyKey(deviceID string, now time.Time) string {
	bucket := now.UTC().Truncate(30 * time.Second).Unix()
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", deviceID, bucket)))
	return fmt.Sprintf("%x", h)
}

// DistributeReward executes the 3-stream split distribution for any reward event.
// It credits the worker, referrer, and treasury based on the total emission.
func DistributeReward(tx *sql.Tx, deviceID string, amount float64, reason string, ikey string) (int64, error) {
	// 1. Look up referrer and their referral count.
	var referrerDeviceID string
	var referrerRefCount int
	_ = tx.QueryRow(
		`SELECT COALESCE(n.referrer_device_id, ''), COALESCE(ref.referral_count, 0)
		 FROM nodes n
		 LEFT JOIN nodes ref ON ref.device_id = n.referrer_device_id
		 WHERE n.device_id = $1`,
		deviceID,
	).Scan(&referrerDeviceID, &referrerRefCount)

	// 2. Compute the three streams with supply-based epoch multiplier + Gear Score.
	// Halving is triggered by total minted supply, not by calendar date.
	currentSupply := getCachedSupply(tx)
	epochMultiplier := CalculateEpochMultiplier(currentSupply)

	// Look up node hardware and identity details for v2 Gear Score multiplier.
	var cpuCores, vramMB, ramMB, bandwidthMbps int
	var deviceType, identityTier string
	var isResidential bool
	var feederTrustScore float64
	var uptimePct float64
	_ = tx.QueryRow(
		`SELECT COALESCE(cpu_cores,0), COALESCE(vram_mb,0), COALESCE(ram_mb,0),
		        COALESCE(bandwidth_mbps,0), COALESCE(device_type,''), COALESCE(is_residential,false),
		        COALESCE(identity_tier, 'anon'), COALESCE(feeder_trust_score, 0.5),
		        COALESCE(uptime_pct, 0.0)
		 FROM nodes WHERE device_id = $1`, deviceID,
	).Scan(&cpuCores, &vramMB, &ramMB, &bandwidthMbps, &deviceType, &isResidential, &identityTier, &feederTrustScore, &uptimePct)

	// V1 HW score
	hwGs := ComputeGearScore(deviceType, cpuCores, vramMB, ramMB, bandwidthMbps, isResidential)
	// V2 GS = 0.4*HW + 0.3*Uptime + 0.2*Quality + 0.1*Feeder_Trust
	// Placeholder: Quality is currently locked at 100% (1000) until the Feeder/Audit cycle
	// provides granular session-ratio data. Uptime is mapped from [0-100] to [0-1000].
	gs := ComputeGearScoreV2(hwGs.Score, int(uptimePct*10), 1000, feederTrustScore)
	rsMult := ComputeRSMultiplier(gs.Score, identityTier)

	// Apply Feeder Bonus: +20% if node provided at least one correct audit report today.
	var correctReports int
	_ = tx.QueryRow(`
		SELECT COUNT(*) FROM feeder_reports 
		WHERE feeder_device_id = $1 AND evaluation_result = 'correct' AND created_at > NOW() - INTERVAL '24 hours'`,
		deviceID,
	).Scan(&correctReports)

	feederBoost := 1.0
	if correctReports > 0 {
		feederBoost = 1.2
	}

	// Apply Sybil subnet penalty
	var nodeIP string
	_ = tx.QueryRow(`SELECT COALESCE(ip,'') FROM nodes WHERE device_id = $1`, deviceID).Scan(&nodeIP)
	sybilPenalty := SybilSubnetPenalty(tx, nodeIP)

	multiplier := epochMultiplier * rsMult * sybilPenalty * feederBoost

	effectiveAmount := amount * multiplier

	// Treasury fee is based on identity tier.
	// Anon: 25%, Peak: 0%.
	treasuryFeeRate := 0.25
	if identityTier == "peak" {
		treasuryFeeRate = 0.0
	}
	refPct := 0.0
	if referrerDeviceID != "" {
		refPct = ReferralPercent(referrerRefCount)
	}
	referralReward := effectiveAmount * refPct
	treasuryReward := effectiveAmount * treasuryFeeRate
	workerReward := effectiveAmount - treasuryReward - referralReward
	if workerReward < 0 {
		workerReward = 0
	}

	// Audit snapshot.
	tier := ReferralTierByCount(referrerRefCount)
	tierName := ""
	if tier != nil {
		tierName = tier.Name
	}
	epochState := CheckCurrentEpoch(currentSupply)
	snapshot := PopRewardSnapshot{
		TotalEmission:    amount,
		WorkerReward:     workerReward,
		ReferralPercent:  refPct,
		ReferralReward:   referralReward,
		ReferrerDeviceID: referrerDeviceID,
		TreasuryReward:   treasuryReward,
		ReferralCount:    referrerRefCount,
		TierName:         tierName,
		ReasonCode:       reason,
		EpochIndex:       epochState.EpochIndex,
		EpochMultiplier:  epochMultiplier,
		GearGrade:        string(gs.Grade),
		GearScore:        gs.Score,
		GearMultiplier:   rsMult,
		TotalMultiplier:  multiplier,
	}
	snapJSON, _ := json.Marshal(snapshot)

	// 3. Insert reward event.
	var eventID int64
	query := `INSERT INTO pop_reward_events
		 (device_id, total_emission, worker_reward, referral_percent, referral_reward,
		  referrer_device_id, treasury_reward, reason_code, policy_snapshot, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), $7, $8, $9::jsonb, NULLIF($10,''))`
	
	if ikey != "" {
		query += " ON CONFLICT (idempotency_key) DO NOTHING"
	}
	query += " RETURNING id"

	err := tx.QueryRow(query, deviceID, amount, workerReward, refPct, referralReward,
		referrerDeviceID, treasuryReward, reason, string(snapJSON), ikey).Scan(&eventID)
	if err != nil {
		return 0, err
	}

	// 4. Record treasury inflow.
	if _, err := tx.Exec(
		`INSERT INTO treasury_ledger (pop_reward_event_id, amount) VALUES ($1, $2)`,
		eventID, treasuryReward,
	); err != nil {
		return 0, fmt.Errorf("DistributeReward treasury_ledger: %w", err)
	}

	// 5. Credit worker earnings.
	if workerReward > 0 {
		if _, err := tx.Exec(
			`INSERT INTO node_earnings (device_id, bytes, earned_usd) VALUES ($1, 0, $2)`,
			deviceID, workerReward,
		); err != nil {
			return 0, fmt.Errorf("DistributeReward node_earnings worker: %w", err)
		}
	}

	// 6. Credit referrer earnings.
	if referrerDeviceID != "" && referralReward > 0 {
		if _, err := tx.Exec(
			`INSERT INTO node_earnings (device_id, bytes, earned_usd) VALUES ($1, 0, $2)`,
			referrerDeviceID, referralReward,
		); err != nil {
			return 0, fmt.Errorf("DistributeReward node_earnings referral: %w", err)
		}
		if _, err := tx.Exec(
			`UPDATE nodes SET total_referral_reward = total_referral_reward + $1 WHERE device_id = $2`,
			referralReward, referrerDeviceID,
		); err != nil {
			return 0, fmt.Errorf("DistributeReward referrer aggregate: %w", err)
		}
	}

	// 7. Queue for daily PEAQ Oracle batch (multi-sig extrinsic).
	if workerReward > 0 {
		if _, err := tx.Exec(
			`INSERT INTO oracle_mint_queue (reward_event_id, device_id, amount_exra, status)
			 SELECT $1, $2, $3, 'pending'`,
			eventID, deviceID, workerReward,
		); err != nil {
			return 0, fmt.Errorf("DistributeReward oracle_mint_queue: %w", err)
		}
	}

	// 8. Update worker aggregate on node.
	if _, err := tx.Exec(
		`UPDATE nodes
		 SET total_worker_reward = total_worker_reward + $1,
		     total_treasury_reward = total_treasury_reward + $2,
		     last_seen = NOW()
		 WHERE device_id = $3`,
		workerReward, treasuryReward, deviceID,
	); err != nil {
		return 0, fmt.Errorf("DistributeReward node aggregate: %w", err)
	}

	log.Printf("[Reward] device_id=%s reason=%s amount=%.8f multiplier=%.1fx worker=%.8f referral=%.8f treasury=%.8f",
		deviceID, reason, effectiveAmount, multiplier, workerReward, referralReward, treasuryReward)

	// Update supply cache (lock already released by getCachedSupply, re-acquire).
	supplyMu.Lock()
	totalMintedCached += effectiveAmount
	supplyMu.Unlock()

	return eventID, nil
}

func getCachedSupply(tx *sql.Tx) float64 {
	supplyMu.Lock()
	defer supplyMu.Unlock()
	// Re-check from DB every 1 minute to stay accurate across instances.
	if time.Since(lastSupplyUpdate) < 1*time.Minute && totalMintedCached > 0 {
		return totalMintedCached
	}
	// Use pure DB connection if tx is nil
	var err error
	var supply float64
	if tx != nil {
		err = tx.QueryRow(`SELECT COALESCE(SUM(amount_exra), 0) FROM oracle_mint_queue`).Scan(&supply)
	} else {
		err = db.DB.QueryRow(`SELECT COALESCE(SUM(amount_exra), 0) FROM oracle_mint_queue`).Scan(&supply)
	}
	
	if err == nil {
		totalMintedCached = supply
		lastSupplyUpdate = time.Now()
	}
	return totalMintedCached
}

// HeartbeatPoP is now non-blocking. It queues the heartbeat for batched processing.
// This is critical for scaling to 50,000 concurrent nodes.
func HeartbeatPoP(deviceID string, totalEmission float64) error {
	select {
	case popChannel <- popRequest{DeviceID: deviceID, Emission: totalEmission}:
		return nil
	default:
		return fmt.Errorf("PoP queue full, dropping heartbeat for %s", deviceID)
	}
}

func StartPopWorker() {
	go func() {
		log.Println("[PoP] Starting background scaling worker...")
		for req := range popChannel {
			// Process heartbeats one by one for now but using a dedicated worker
			// to keep hub goroutines free. 
			// Full batch SQL optimization would require record buffering here.
			processHeartbeatSynchronous(req.DeviceID, req.Emission)
		}
	}()
}

func processHeartbeatSynchronous(deviceID string, totalEmission float64) {
	now := time.Now().UTC()
	ikey := idempotencyKey(deviceID, now)

	tx, err := db.DB.Begin()
	if err != nil {
		log.Printf("[PoP] worker error (Begin): %v", err)
		return
	}
	defer tx.Rollback()

	_, err = DistributeReward(tx, deviceID, totalEmission, "pop_heartbeat", ikey)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[PoP] worker distribution failed for %s: %v", deviceID, err)
		}
		return
	}

	if _, err := tx.Exec(`UPDATE nodes SET last_heartbeat = NOW(), active = true, status = 'online' WHERE device_id = $1`, deviceID); err != nil {
		log.Printf("[PoP] worker node update failed for %s: %v", deviceID, err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[PoP] worker commit failed: %v", err)
		return
	}

	// ── Marketplace v2.1: Redis Sync ──
	if globalHub != nil {
		syncNodeToRedis(deviceID)
		
		// ── Anti-Fraud: Periodic Feeder Assignment (5% probability) ──
		if rand.Float64() < 0.05 {
			go func(targetID string) {
				assignment, err := AssignFeeder(targetID)
				if err != nil {
					// No feeders available or other error, skip silently
					return
				}
				
				// Lookup target IP and port for the feeder to check
				var targetIP string
				var targetPort int
				err = db.DB.QueryRow(`SELECT ip, COALESCE(port, 8080) FROM nodes WHERE device_id = $1`, targetID).Scan(&targetIP, &targetPort)
				if err != nil {
					return
				}
				
				log.Printf("[Feeder] Assigned %s to audit %s (assignment_id=%d)", assignment.FeederDeviceID, targetID, assignment.ID)
				globalHub.BroadcastFeederTask(assignment.FeederDeviceID, assignment.ID, targetID, targetIP, targetPort)
			}(deviceID)
		}
	}
}

func syncNodeToRedis(deviceID string) {
	var n Node
	var rsMult float64
	err := db.DB.QueryRow(`
		SELECT device_id, country, device_tier, is_residential, status, bandwidth_mbps, 
		       cpu_cores, vram_mb, ram_mb, rs_mult, COALESCE(uptime_pct, 0.0)
		FROM nodes WHERE device_id = $1`, deviceID).Scan(
		&n.DeviceID, &n.Country, &n.DeviceTier, &n.IsResidential, &n.Status, &n.BandwidthMbps,
		&n.CPUCores, &n.VRAMMB, &n.RAMMB, &rsMult, &n.Uptime,
	)
	if err != nil {
		return
	}

	if rsMult <= 0 {
		rsMult = 1.0
	}

	// Blueprint formula: base := 0.30 / rsMult
	priceGB := 0.30 / rsMult
	
	// Determine RS Tier (A, B, C) based on rsMult
	rsTier := "C"
	if rsMult >= 1.5 {
		rsTier = "A"
	} else if rsMult >= 1.0 {
		rsTier = "B"
	}

	rsScore := rsMult * 500.0
	if rsScore > 1000 {
		rsScore = 1000
	}

	pubNode := PublicNode{
		ID:            n.DeviceID,
		Country:       n.Country,
		DeviceTier:    n.DeviceTier,
		IsResidential: n.IsResidential,
		Status:        n.Status,
		BandwidthMbps: n.BandwidthMbps,
		CPUCores:      n.CPUCores,
		VRAMMB:        n.VRAMMB,
		RAMMB:         n.RAMMB,
		PricePerGB:    priceGB,
		RSScore:       rsScore,
		Uptime:        n.Uptime / 100.0, // Convert [0-100] to [0.0-1.0]
		RSTier:        rsTier,
		LastSeen:      time.Now(),
	}

	pubNodeJSON, _ := json.Marshal(pubNode)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Sync both country-specific and global list
	globalHub.SyncNodeToRedis(ctx, n.Country, rsTier, priceGB, string(pubNodeJSON))
	globalHub.SyncNodeToRedis(ctx, "ALL", rsTier, priceGB, string(pubNodeJSON))
}

// SetReferrer links a node to its referrer. Safe to call multiple times;
// only sets the referrer if the node doesn't already have one (idempotent).
// Also increments the referrer's referral_count.
func SetReferrer(deviceID, referrerDeviceID string) error {
	if deviceID == "" || referrerDeviceID == "" || deviceID == referrerDeviceID {
		return fmt.Errorf("invalid referrer relationship: device_id=%s referrer=%s", deviceID, referrerDeviceID)
	}
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Only set if not already set (one-time binding).
	var existing string
	_ = tx.QueryRow(`SELECT COALESCE(referrer_device_id,'') FROM nodes WHERE device_id = $1`, deviceID).Scan(&existing)
	if existing != "" {
		return nil // already linked, idempotent
	}

	if _, err := tx.Exec(
		`UPDATE nodes SET referrer_device_id = $1 WHERE device_id = $2`,
		referrerDeviceID, deviceID,
	); err != nil {
		return fmt.Errorf("SetReferrer update node: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE nodes SET referral_count = referral_count + 1 WHERE device_id = $1`,
		referrerDeviceID,
	); err != nil {
		return fmt.Errorf("SetReferrer update referral_count: %w", err)
	}
	return tx.Commit()
}

