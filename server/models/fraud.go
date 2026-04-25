package models

// fraud.go — Anti-fraud engine for the EXRA network.
//
// Implements four layers of protection:
//   1. Sybil detection  — /24 subnet density check (Gear Score penalty)
//   2. Reliability gate — uptime_pct < 95% blocks high-volume orders
//   3. Mint circuit breaker — hourly withdrawal volume > 5% pauses minting 30 min
//   4. Traffic probe  — oracle sends a known payload; hash mismatch = instant freeze
//
// FreezeNode() is the terminal action: active=false, balance zeroed, freeze_reason logged.
// Frozen nodes are permanently excluded from reward distribution.

import (
	crand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"exra/db"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// ── Sybil / IP density ───────────────────────────────────────────────────────

// SybilSubnetPenalty returns the Gear Score multiplier reduction for a node
// based on how many active nodes share its /24 subnet (IPv4) or /48 subnet (IPv6).
//
//	< 3 nodes on subnet  → no penalty (1.0)
//	3–9 nodes on subnet  → 0.5× (datacenter cluster, but could be valid)
//	10+ nodes on subnet  → 0.1× (almost certainly a farm)
func SybilSubnetPenalty(tx *sql.Tx, ip string) float64 {
	if ip == "" {
		return 1.0
	}
	subnet, isIPv6 := toSubnetPrefix(ip)
	if subnet == "" {
		return 1.0
	}

	var query string
	var arg interface{}
	if isIPv6 {
		// Use PostgreSQL inet containment: works regardless of how IPv6 is stored.
		query = `SELECT COUNT(*) FROM nodes
			 WHERE active = true
			   AND inet(ip) << $1::inet
			   AND last_heartbeat > NOW() - INTERVAL '10 minutes'`
		arg = subnet // "2001:db8::/48"
	} else {
		query = `SELECT COUNT(*) FROM nodes
			 WHERE active = true
			   AND ip LIKE $1
			   AND last_heartbeat > NOW() - INTERVAL '10 minutes'`
		arg = subnet + "%" // "1.2.3.%"
	}

	var count int
	var err error
	if tx != nil {
		err = tx.QueryRow(query, arg).Scan(&count)
	} else {
		err = db.DB.QueryRow(query, arg).Scan(&count)
	}

	if err != nil {
		return 1.0
	}
	switch {
	case count >= 10:
		return 0.1 // farm penalty
	case count >= 3:
		return 0.5 // cluster warning
	default:
		return 1.0
	}
}

// toSubnetPrefix extracts the subnet prefix used for Sybil detection.
// IPv4: returns "/24" dotted prefix "1.2.3." and isIPv6=false.
// IPv6: returns "/48" CIDR "2001:db8::/48" and isIPv6=true.
func toSubnetPrefix(ip string) (prefix string, isIPv6 bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", false
	}
	if v4 := parsed.To4(); v4 != nil {
		parts := strings.Split(v4.String(), ".")
		return fmt.Sprintf("%s.%s.%s.", parts[0], parts[1], parts[2]), false
	}
	mask := net.CIDRMask(48, 128)
	network := parsed.Mask(mask)
	return fmt.Sprintf("%s/48", net.IP(network).String()), true
}

// ── Node freeze ──────────────────────────────────────────────────────────────

// FreezeNode permanently deactivates a node and zeros its earned balance.
// reason is stored for audit. DeviceID is permanently banned.
func FreezeNode(deviceID, reason string) error {
	log.Printf("[Fraud] FREEZE device_id=%s reason=%s", deviceID, reason)
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark node frozen.
	_, err = tx.Exec(`
		UPDATE nodes
		SET active = false,
		    status = 'frozen',
		    freeze_reason = $2,
		    frozen_at = NOW(),
		    updated_at = NOW()
		WHERE device_id = $1`,
		deviceID, reason,
	)
	if err != nil {
		return err
	}

	// Zero out earned balance (burn the balance as penalty).
	_, err = tx.Exec(`
		INSERT INTO node_earnings (device_id, bytes, earned_usd)
		SELECT $1, 0, -COALESCE(SUM(earned_usd), 0)
		FROM node_earnings
		WHERE device_id = $1`,
		deviceID,
	)
	if err != nil {
		return err
	}

	// Cancel any pending payout requests.
	_, err = tx.Exec(`
		UPDATE payout_requests
		SET status = 'rejected', updated_at = NOW()
		WHERE device_id = $1 AND status = 'pending'`,
		deviceID,
	)
	if err != nil {
		return err
	}

	// Cancel pending oracle mints.
	_, err = tx.Exec(`
		UPDATE oracle_mint_queue
		SET status = 'dlq', dlq_reason = $2, updated_at = NOW()
		WHERE device_id = $1 AND status IN ('pending','retryable')`,
		deviceID, "node_frozen: "+reason,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// IsNodeFrozen returns true if a device is permanently frozen.
func IsNodeFrozen(deviceID string) bool {
	var status string
	err := db.DB.QueryRow(
		`SELECT COALESCE(status,'') FROM nodes WHERE device_id = $1`, deviceID,
	).Scan(&status)
	return err == nil && status == "frozen"
}

// ── Traffic probe ────────────────────────────────────────────────────────────

type ProbeResult string

const (
	ProbePass    ProbeResult = "pass"
	ProbeFail    ProbeResult = "fail"
	ProbePending ProbeResult = "pending"
)

// RecordProbe stores an outbound probe for a device in oracle_probes table.
// The oracle sends a known payload and expects the sha256 hash back.
func RecordProbe(deviceID, probeID, expectedHash string) error {
	_, err := db.DB.Exec(`
		INSERT INTO oracle_probes (device_id, probe_id, expected_hash, result, created_at)
		VALUES ($1, $2, $3, 'pending', NOW())
		ON CONFLICT (probe_id) DO NOTHING`,
		deviceID, probeID, expectedHash,
	)
	return err
}

// VerifyProbe checks a probe response. Returns false + freezes node on mismatch.
func VerifyProbe(deviceID, probeID, responseHash string) bool {
	var expectedHash string
	err := db.DB.QueryRow(`
		SELECT expected_hash FROM oracle_probes
		WHERE probe_id = $1 AND device_id = $2 AND result = 'pending'
		  AND created_at > NOW() - INTERVAL '5 minutes'`,
		probeID, deviceID,
	).Scan(&expectedHash)
	if err != nil {
		log.Printf("[Probe] probe not found or expired: probe_id=%s device_id=%s", probeID, deviceID)
		return false
	}

	if responseHash != expectedHash {
		log.Printf("[Probe] MISMATCH device_id=%s probe_id=%s expected=%s got=%s → FREEZE",
			deviceID, probeID, expectedHash, responseHash)
		_, _ = db.DB.Exec(`
			UPDATE oracle_probes SET result='fail', responded_at=NOW() WHERE probe_id=$1`,
			probeID,
		)
		_ = FreezeNode(deviceID, "probe_hash_mismatch")
		return false
	}

	_, _ = db.DB.Exec(`
		UPDATE oracle_probes SET result='pass', responded_at=NOW() WHERE probe_id=$1`,
		probeID,
	)
	return true
}

// ── Mint circuit breaker ─────────────────────────────────────────────────────

var (
	mintCBMu       sync.Mutex
	mintPausedUntil time.Time
	mintCBReason    string
)

// MintCircuitBreakerState returns whether minting is currently paused and until when.
func MintCircuitBreakerState() (paused bool, until time.Time, reason string) {
	mintCBMu.Lock()
	defer mintCBMu.Unlock()
	return time.Now().Before(mintPausedUntil), mintPausedUntil, mintCBReason
}

// CheckMintCircuitBreaker returns false if hourly withdrawal EXRA volume exceeds
// 5% of total liquidity (circulating supply). Pause lasts 30 minutes.
func CheckMintCircuitBreaker() (allowed bool, reason string) {
	mintCBMu.Lock()
	defer mintCBMu.Unlock()

	now := time.Now()
	if now.Before(mintPausedUntil) {
		return false, mintCBReason
	}

	// Hourly mint volume (submitted/confirmed in last 60 minutes)
	var hourlyMinted float64
	_ = db.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_exra), 0)
		FROM oracle_mint_queue
		WHERE status IN ('submitted','confirmed','minted')
		  AND created_at > NOW() - INTERVAL '1 hour'`,
	).Scan(&hourlyMinted)

	// Total circulating supply
	var totalMinted float64
	_ = db.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_exra), 0)
		FROM oracle_mint_queue
		WHERE status IN ('submitted','confirmed','minted')`,
	).Scan(&totalMinted)

	if totalMinted > 0 && hourlyMinted/totalMinted > 0.05 {
		mintPausedUntil = now.Add(30 * time.Minute)
		mintCBReason = fmt.Sprintf(
			"circuit breaker: hourly mint %.2f = %.1f%% of supply %.2f (> 5%%) — paused 30 min",
			hourlyMinted, 100*hourlyMinted/totalMinted, totalMinted,
		)
		log.Printf("[MintCB] TRIGGERED: %s", mintCBReason)
		return false, mintCBReason
	}

	return true, ""
}

// ── Reliability gate ─────────────────────────────────────────────────────────

// MinUptimeForHighVolume is the minimum uptime percentage a node must maintain
// to participate in orders above ReliabilityGateGB gigabytes.
const (
	MinUptimeForHighVolume = 95.0  // percent
	ReliabilityGateGB      = 10.0  // GB threshold
)

// NodeMeetsReliabilityGate returns true if the node is allowed to service
// an order of the given size (in GB).
func NodeMeetsReliabilityGate(deviceID string, requestedGB float64) bool {
	if requestedGB < ReliabilityGateGB {
		return true // small orders: anyone can serve
	}
	var uptimePct float64
	err := db.DB.QueryRow(
		`SELECT COALESCE(uptime_pct, 0) FROM nodes WHERE device_id = $1`, deviceID,
	).Scan(&uptimePct)
	if err != nil {
		return false
	}
	return uptimePct >= MinUptimeForHighVolume
}

// ── Canary Tasks (MVP-4) ──────────────────────────────────────────────────

type CanaryTask struct {
	ID              int64      `json:"id"`
	DeviceID        string     `json:"device_id"`
	TaskType        string     `json:"task_type"`
	ExpectedResult  string     `json:"expected_result"`
	SubmittedResult *string    `json:"submitted_result"`
	InjectedAt      time.Time  `json:"injected_at"`
	RespondedAt     *time.Time `json:"responded_at"`
	Result          string     `json:"result"`
	PenaltyApplied  bool       `json:"penalty_applied"`
}

// ShouldInjectCanary returns true with a 5% probability
func ShouldInjectCanary(deviceID string) bool {
	return rand.Float64() <= 0.05
}

// CreateCanaryTask creates a mock fake task pointing at a known oracle validation endpoint
func CreateCanaryTask(deviceID string) (*CanaryTask, error) {
	var count int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM canary_tasks WHERE device_id = $1 AND result = 'pending'`, deviceID).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("active canary task already exists")
	}

	// AUDIT §1 E2: previously the literal "canary_expected_hash" was the
	// expected hash for EVERY canary task on EVERY device. Any worker that
	// knew the constant (it leaked via source and past traces) bypassed the
	// check trivially. We now generate a per-task 32-byte nonce and derive
	// the expected hash as sha256(nonce || deviceID) so:
	//   * no two canary tasks share the same expected_result
	//   * the value is not a compile-time literal
	// Proper end-to-end design still requires the Oracle to pin the nonce
	// to a specific proxy-challenge payload so the worker has to actually
	// perform the task to reproduce the hash; this change is the minimum
	// on-server fix to stop the universal-token attack.
	nonce := make([]byte, 32)
	if _, err := crand.Read(nonce); err != nil {
		return nil, fmt.Errorf("canary nonce: %w", err)
	}
	sum := sha256.Sum256(append(nonce, []byte(deviceID)...))
	expectedResult := hex.EncodeToString(sum[:])

	task := &CanaryTask{}
	err = db.DB.QueryRow(`
		INSERT INTO canary_tasks (device_id, task_type, expected_result, result)
		VALUES ($1, 'proxy_hash', $2, 'pending')
		RETURNING id, device_id, task_type, expected_result, result, injected_at`,
		deviceID, expectedResult,
	).Scan(&task.ID, &task.DeviceID, &task.TaskType, &task.ExpectedResult, &task.Result, &task.InjectedAt)
	if err != nil {
		return nil, err
	}
	return task, nil
}

// VerifyCanaryResult checks the reported result against expected
func VerifyCanaryResult(deviceID string, taskID int64, submittedResult string) bool {
	var expected string
	err := db.DB.QueryRow(`SELECT expected_result FROM canary_tasks WHERE id = $1 AND device_id = $2 AND result = 'pending'`, taskID, deviceID).Scan(&expected)
	if err != nil {
		return false
	}

	pass := (submittedResult == expected)
	status := "fail"
	if pass {
		status = "pass"
	}

	db.DB.Exec(`UPDATE canary_tasks SET result = $1, submitted_result = $2, responded_at = NOW() WHERE id = $3`, status, submittedResult, taskID)

	if !pass {
		BurnDayCredits(deviceID)
	}
	return pass
}

// BurnDayCredits zeroes earnings generated in the last 24h & kills reputation mult immediately.
func BurnDayCredits(deviceID string) {
	tx, err := db.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	// 1. Burn day credits and count the total burned
	var burnedUSD float64
	tx.QueryRow(`
		WITH burned AS (
			DELETE FROM node_earnings
			WHERE device_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			RETURNING earned_usd
		)
		SELECT COALESCE(SUM(earned_usd), 0) FROM burned
	`, deviceID).Scan(&burnedUSD)
	
	log.Printf("[Canary] PENALTY: Burned %.6f USD for device_id=%s", burnedUSD, deviceID)

	// 1b. Burn pending tokens from oracle_mint_queue generated in the last 24h
	tx.Exec(`
		DELETE FROM oracle_mint_queue
		WHERE device_id = $1 AND status = 'pending' AND created_at > NOW() - INTERVAL '24 hours'
	`, deviceID)

	// 2. Set Reputation Score and continuous trust to 0
	tx.Exec(`UPDATE nodes SET honest_days_streak = 0, rs_mult = 0 WHERE device_id = $1`, deviceID)

	// 3. Mark penalty applied
	tx.Exec(`UPDATE canary_tasks SET penalty_applied = true WHERE device_id = $1 AND result = 'fail' AND penalty_applied = false`, deviceID)

	tx.Commit()
}
