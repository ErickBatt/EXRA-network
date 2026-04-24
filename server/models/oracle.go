package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"exra/db"
	"exra/peaq"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChainSafe/go-schnorrkel"
	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

// usdToPlancks converts a USD float64 amount to plancks (1e9 per USD).
// math.Round is required: bare uint64 cast truncates, so 0.1*1e9 = 99_999_999
// instead of 100_000_000, causing every tenth-cent payout to be 1 planck short.
func usdToPlancks(usd float64) uint64 {
	return uint64(math.Round(usd * 1_000_000_000))
}

// verifyProposalSignature validates an sr25519 signature produced by
// signature.Sign(hashBytes, seed) in SaveOracleProposal. Verifies that
// `sigHex` is signed by `oracleID` (hex-encoded 32-byte public key) over
// the hex-decoded `payloadHash`.
//
// AUDIT §1 F1: the previous consensus path stored signatures verbatim and
// never verified them, so any peer could forge oracle_signatures rows for
// arbitrary oracleIDs and reach 2/3 consensus by brute adversarial count.
// We now reject any proposal whose signature does not verify.
//
// Implemented inline (not via middleware.VerifyDIDSignature) to avoid a
// circular import between models → middleware → models.
func verifyProposalSignature(payloadHash, oracleID, sigHex string) bool {
	if payloadHash == "" || oracleID == "" || sigHex == "" {
		return false
	}
	sigBytes, err := hex.DecodeString(strings.TrimPrefix(sigHex, "0x"))
	if err != nil || len(sigBytes) != 64 {
		return false
	}
	pubBytes, err := hex.DecodeString(strings.TrimPrefix(oracleID, "0x"))
	if err != nil || len(pubBytes) != 32 {
		return false
	}
	msg, err := hex.DecodeString(payloadHash)
	if err != nil {
		return false
	}

	var sigArr [64]byte
	copy(sigArr[:], sigBytes)
	var pubArr [32]byte
	copy(pubArr[:], pubBytes)

	publicKey, err := schnorrkel.NewPublicKey(pubArr)
	if err != nil {
		return false
	}
	sig := &schnorrkel.Signature{}
	if err := sig.Decode(sigArr); err != nil {
		return false
	}
	ctx := schnorrkel.NewSigningContext([]byte("substrate"), msg)
	ok, err := publicKey.Verify(sig, ctx)
	if err != nil {
		log.Printf("[Oracle] F1 verify error oracle=%s: %v", oracleID, err)
		return false
	}
	return ok
}

var (
	peaqClient peaq.BlockchainClient
	once       sync.Once
)

// SetPeaqClient allows manual injection of a peaq client (primarily for testing)
func SetPeaqClient(c peaq.BlockchainClient) {
	peaqClient = c
}

// InitPeaq enables blockchain interaction for the oracle
func InitPeaq() {
	client, err := peaq.InitPeaqClient()
	if err != nil {
		log.Printf("[Oracle] WARNING: Peaq integration disabled: %v", err)
		return
	}
	peaqClient = client
	log.Println("[Oracle] Peaq blockchain client initialized.")
}

// OracleBatch represents a daily collection of node earnings.
type OracleBatch struct {
	ID           int64           `json:"id"`
	BatchDate    time.Time       `json:"batch_date"`
	OracleID     string          `json:"oracle_id"`
	TotalCredits float64         `json:"total_credits"`
	DIDCount     int             `json:"did_count"`
	PayloadHash  string          `json:"payload_hash"`
	Status       string          `json:"status"` // received, consensus, applied, disputed
	BatchJSON    json.RawMessage `json:"batch_json"`
}

// OracleProposal is the broadcasted message for 2/3 consensus.
type OracleProposal struct {
	BatchDate   string  `json:"batch_date"` // YYYY-MM-DD
	PayloadHash string  `json:"payload_hash"`
	OracleID    string  `json:"oracle_id"`
	TotalAmount float64 `json:"total_amount"`
	Signature   string  `json:"signature"` // [NEW] sr25519 signature of PayloadHash
}

// CalculateDailyDistribution sums node_earnings for a specific 24h period.
// Optimized for 50k+ nodes using spatial/temporal indexing.
func CalculateDailyDistribution(date time.Time) (map[string]float64, error) {
	dateStr := date.Format("2006-01-02")
	log.Printf("[Oracle] Aggregating earnings for date=%s (High-Scale Mode)", dateStr)

	// We use the new idx_node_earnings_aggregation
	rows, err := db.DB.Query(`
		SELECT n.did, SUM(e.earned_usd)
		FROM node_earnings e
		JOIN nodes n ON e.device_id = n.device_id
		WHERE e.batch_id IS NULL
		  AND n.did IS NOT NULL
		  AND e.created_at >= $1::TIMESTAMP
		  AND e.created_at < ($1::TIMESTAMP + INTERVAL '1 day')
		GROUP BY n.did
		ORDER BY n.did ASC
	`, dateStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dist := make(map[string]float64)
	for rows.Next() {
		var did string
		var amount float64
		if err := rows.Scan(&did, &amount); err != nil {
			return nil, err
		}
		dist[did] = amount
	}

	return dist, nil
}

// HashDistribution generates a deterministic SHA256 hash of the distribution map.
func HashDistribution(dist map[string]float64) string {
	// 1. Sort DIDs to ensure deterministic JSON
	keys := make([]string, 0, len(dist))
	for k := range dist {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	type entry struct {
		DID    string  `json:"did"`
		Amount float64 `json:"amount"`
	}
	entries := make([]entry, 0, len(dist))
	for _, k := range keys {
		entries = append(entries, entry{DID: k, Amount: dist[k]})
	}

	b, _ := json.Marshal(entries)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

// SaveOracleProposal records a local proposal from this oracle instance.
func SaveOracleProposal(date time.Time, oracleID, hash string, dist map[string]float64) (string, error) {
	distJSON, _ := json.Marshal(dist)
	total := 0.0
	for _, a := range dist {
		total += a
	}

	// 1. Sign the hash (Mandatory for DePIN consensus)
	seed := os.Getenv("PEAQ_ORACLE_SEED")
	if seed == "" {
		return "", fmt.Errorf("PEAQ_ORACLE_SEED is mandatory for oracle consensus")
	}

	kp, err := signature.KeyringPairFromSecret(seed, 42)
	if err != nil {
		return "", fmt.Errorf("failed to derive keyring: %v", err)
	}

	msg, _ := hex.DecodeString(hash)
	s, err := signature.Sign(msg, kp.URI)
	if err != nil {
		return "", fmt.Errorf("failed to sign payload: %v", err)
	}
	sigStr := hex.EncodeToString(s)

	dateStr := date.Format("2006-01-02")
	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var batchID int64
	err = tx.QueryRow(`
		INSERT INTO oracle_batches (batch_date, oracle_id, total_credits, payload_hash, batch_json, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (batch_date, oracle_id) DO UPDATE
		SET payload_hash = EXCLUDED.payload_hash, 
		    batch_json = EXCLUDED.batch_json,
		    total_credits = EXCLUDED.total_credits,
		    updated_at = NOW()
		RETURNING id
	`, dateStr, oracleID, total, hash, distJSON, "received").Scan(&batchID)

	if err != nil {
		return "", err
	}

	// 2. Save signature to oracle_signatures
	_, err = tx.Exec(`
		INSERT INTO oracle_signatures (batch_id, oracle_did, signature)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, batchID, oracleID, sigStr)

	if err != nil {
		return "", err
	}

	return sigStr, tx.Commit()
}

// ProcessOracleProposal is called when a peer oracle broadcasts its payload hash.
func ProcessOracleProposal(prop OracleProposal, oracleNodes int) {
	log.Printf("[Oracle] Received proposal from %s for %s: %s", prop.OracleID, prop.BatchDate, prop.PayloadHash)

	// AUDIT §1 F1: verify the sr25519 signature before persisting anything.
	// Rejecting forged proposals early prevents them from counting toward
	// the 2/3 threshold and prevents unbounded rows in oracle_signatures.
	if !verifyProposalSignature(prop.PayloadHash, prop.OracleID, prop.Signature) {
		log.Printf("[Oracle] Rejecting proposal from %s: invalid signature (batch=%s)", prop.OracleID, prop.BatchDate)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		log.Printf("[Oracle] Failed to start transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// 1. Record the peer's proposal in our DB
	var batchID int64
	err = tx.QueryRow(`
		INSERT INTO oracle_batches (batch_date, oracle_id, total_credits, payload_hash, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (batch_date, oracle_id) DO UPDATE
		SET payload_hash = EXCLUDED.payload_hash, 
		    status = EXCLUDED.status,
		    updated_at = NOW()
		RETURNING id
	`, prop.BatchDate, prop.OracleID, prop.TotalAmount, prop.PayloadHash, "received").Scan(&batchID)

	if err != nil {
		log.Printf("[Oracle] Failed to save peer proposal: %v", err)
		return
	}

	// 2. Save signature
	_, err = tx.Exec(`
		INSERT INTO oracle_signatures (batch_id, oracle_did, signature)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, batchID, prop.OracleID, prop.Signature)

	if err != nil {
		log.Printf("[Oracle] Failed to save peer signature: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[Oracle] Failed to commit peer proposal: %v", err)
		return
	}

	// 3. Check if we reached 2/3 consensus for this hash
	CheckOracleConsensus(prop.BatchDate, prop.PayloadHash, oracleNodes)
}

// CheckOracleConsensus verifies if 2/3 oracles agree on a specific payload hash.
func CheckOracleConsensus(batchDate, hash string, totalNodes int) {
	var count int
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM oracle_batches WHERE batch_date = $1 AND payload_hash = $2 AND status = 'received'`,
		batchDate, hash).Scan(&count)

	// ceil(2n/3) — mirrors pallet-exra v2.4.1 verify_oracle_multisig threshold.
	// Floor (n*2)/3 under-counts: N=5 gives 3 (Go) vs 4 (Rust), N=7 gives 4 vs 5.
	threshold := (totalNodes*2 + 2) / 3
	if threshold < 2 {
		threshold = 2
	}

	if count >= threshold {
		log.Printf("[Oracle] CONSENSUS REACHED for %s (hash: %s, nodes: %d/%d)", batchDate, hash, count, totalNodes)

		// 1. Mark batch as consensus
		_, err := db.DB.Exec(`UPDATE oracle_batches SET status = 'consensus' WHERE batch_date = $1 AND payload_hash = $2`,
			batchDate, hash)

		if err != nil {
			log.Printf("[Oracle] Consensus update failed: %v", err)
			return
		}

		// 2. Trigger on-chain minting
		TriggerBatchMint(batchDate, hash)

		// 3. Trigger on-chain reputation update (Phase 2 DePIN)
		TriggerReputationBatch(batchDate, hash)
	}
}

// RunOracleWorker runs a background loop that triggers daily batching at 00:00 UTC.
// Multi-oracle consensus is coordinated via Redis.
func RunOracleWorker(oracleNodes int, hubBroadcast func(OracleProposal)) {
	seed := os.Getenv("PEAQ_ORACLE_SEED")
	if seed == "" {
		log.Fatal("[Oracle] FATAL: PEAQ_ORACLE_SEED environment variable not set. Oracle cannot start.")
	}

	kp, err := signature.KeyringPairFromSecret(seed, 42)
	if err != nil {
		log.Fatalf("[Oracle] FATAL: Failed to derive oracle identity: %v", err)
	}
	oracleID := hex.EncodeToString(kp.PublicKey)

	log.Printf("[Oracle] Starting %s (nodes: %d)", oracleID, oracleNodes)

	// Check for pending/failed batches on startup
	go processMissedBatches(oracleID, oracleNodes, hubBroadcast)

	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		now := time.Now().UTC()
		// At 00:00 UTC, we calculate the batch for the PREVIOUS day.
		if now.Hour() == 0 {
			yesterday := now.AddDate(0, 0, -1)
			processDay(yesterday, oracleID, oracleNodes, hubBroadcast)
		}
	}
}

func processDay(date time.Time, oracleID string, oracleNodes int, hubBroadcast func(OracleProposal)) {
	dateStr := date.Format("2006-01-02")
	
	dist, err := CalculateDailyDistribution(date)
	if err != nil {
		log.Printf("[Oracle] Distribution calculation failed for %s: %v", dateStr, err)
		return
	}
	if len(dist) == 0 {
		log.Printf("[Oracle] No earnings found for %s, skipping batch.", dateStr)
		return
	}

	hash := HashDistribution(dist)
	log.Printf("[Oracle] Proposal for %s: nodes=%d hash=%s", dateStr, len(dist), hash)

	// 1. Save locally and Sign
	sig, err := SaveOracleProposal(date, oracleID, hash, dist)
	if err != nil {
		log.Printf("[Oracle] Failed to save local proposal: %v", err)
		return
	}

	// 2. Broadcast to peers
	total := 0.0
	for _, a := range dist {
		total += a
	}
	hubBroadcast(OracleProposal{
		BatchDate:   dateStr,
		PayloadHash: hash,
		OracleID:    oracleID,
		TotalAmount: total,
		Signature:   sig,
	})

	// 3. Immediately check if we are the only oracle or reached consensus
	CheckOracleConsensus(dateStr, hash, oracleNodes)
}

func processMissedBatches(oracleID string, oracleNodes int, hubBroadcast func(OracleProposal)) {
	// Simple lookup for yesterday just in case we started after 00:00
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	processDay(yesterday, oracleID, oracleNodes, hubBroadcast)
}

// TriggerBatchMint triggers the actual on-chain minting after consensus.
func TriggerBatchMint(batchDate string, hash string) {
	if peaqClient == nil {
		log.Printf("[Oracle] Skip batch mint: Peaq client not initialized.")
		return
	}

	log.Printf("[PEAQ] >>> BATCH MINT TRIGGERED <<< (Consensus Hash: %s)", hash)

	// 1. Fetch batch and rewards
	var batchID int64
	var batchJSON []byte
	err := db.DB.QueryRow(`
		SELECT id, batch_json FROM oracle_batches 
		WHERE batch_date = $1 AND payload_hash = $2 AND status = 'consensus' 
		LIMIT 1
	`, batchDate, hash).Scan(&batchID, &batchJSON)
	
	if err != nil {
		log.Printf("[Oracle] Failed to find consensus batch: %v", err)
		return
	}

	var dist map[string]float64
	json.Unmarshal(batchJSON, &dist)

	// 2. Fetch all collected signatures for this hash
	rows, err := db.DB.Query(`
		SELECT oracle_did, signature FROM oracle_signatures WHERE batch_id IN (
			SELECT id FROM oracle_batches WHERE batch_date = $1 AND payload_hash = $2
		)
	`, batchDate, hash)
	if err != nil {
		log.Printf("[Oracle] Failed to fetch signatures: %v", err)
		return
	}
	defer rows.Close()

	// 2a. Fetch on-chain OracleSet to map DID → index for IndexedSignature (v2.4.1).
	oracleKeys, err := peaqClient.GetOracleSet()
	if err != nil {
		log.Printf("[Oracle] GetOracleSet failed: %v — aborting batch mint", err)
		return
	}
	oracleIndexMap := make(map[[32]byte]uint8, len(oracleKeys))
	for i, key := range oracleKeys {
		oracleIndexMap[key] = uint8(i)
	}

	var sigs []peaq.IndexedSignature
	for rows.Next() {
		var did, sigStr string
		if err := rows.Scan(&did, &sigStr); err == nil {
			pubBytes, hexErr := hex.DecodeString(strings.TrimPrefix(did, "0x"))
			if hexErr != nil || len(pubBytes) != 32 {
				continue
			}
			var pubKey [32]byte
			copy(pubKey[:], pubBytes)
			idx, ok := oracleIndexMap[pubKey]
			if !ok {
				log.Printf("[Oracle] oracle DID %s not in on-chain OracleSet, skipping sig", did)
				continue
			}
			sigBytes, _ := hex.DecodeString(sigStr)
			var fixedSig [64]byte
			copy(fixedSig[:], sigBytes)
			sigs = append(sigs, peaq.IndexedSignature{
				Index:     idx,
				Signature: fixedSig,
			})
		}
	}

	// 3. Prepare claims (Vec<Claim{account, net}> for pallet-exra v2.4.1 batch_mint).
	var claims []peaq.ClaimEntry
	for did, amount := range dist {
		acc, err := types.NewAccountIDFromHexString(did)
		if err == nil {
			val := usdToPlancks(amount)
			claims = append(claims, peaq.ClaimEntry{
				Account: *acc,
				Net:     types.NewU128(*big.NewInt(0).SetUint64(val)),
			})
		}
	}

	// 4. batchID as H256: sha256 of the batch date string for determinism.
	batchIDBytes := sha256.Sum256([]byte(batchDate))

	// 5. Send Extrinsic
	txHash, err := peaqClient.SendBatchMint(batchIDBytes, claims, sigs)
	if err != nil {
		log.Printf("[Oracle] On-chain mint failed: %v", err)
		return
	}

	log.Printf("[PEAQ] Successfully submitted batch_mint tx: %s", txHash)

	// 5. Finalize in DB
	_, err = db.DB.Exec(`
		UPDATE node_earnings
		SET batch_id = $1
		WHERE batch_id IS NULL
		  AND created_at >= $2::TIMESTAMP
		  AND created_at < ($2::TIMESTAMP + INTERVAL '1 day')
	`, batchID, batchDate)

	if err != nil {
		log.Printf("[Oracle] Critical: Failed to finalize earnings: %v", err)
		return
	}
	
	_, _ = db.DB.Exec(`
		UPDATE oracle_batches 
		SET status = 'applied', applied_at = NOW(), is_finalized_on_chain = TRUE, on_chain_tx_hash = $1
		WHERE id = $2
	`, txHash, batchID)
}

// TriggerReputationBatch syncs node ratings to the blockchain
func TriggerReputationBatch(batchDate string, hash string) {
	if peaqClient == nil {
		return
	}

	log.Printf("[PEAQ] >>> REPUTATION SYNC TRIGGERED <<<")

	// 1. Fetch scores for all active DID-enabled nodes
	rows, err := db.DB.Query(`
		SELECT did, rs_score FROM nodes WHERE did IS NOT NULL AND active = true
	`)
	if err != nil {
		log.Printf("[Oracle] Reputation fetch failed: %v", err)
		return
	}
	defer rows.Close()

	var entries []peaq.StatEntry
	for rows.Next() {
		var did string
		var score float64
		if err := rows.Scan(&did, &score); err == nil {
			acc, err := types.NewAccountIDFromHexString(did)
			if err == nil {
				entries = append(entries, peaq.StatEntry{
					Account:    *acc,
					Heartbeats: 0, // populated by PoP heartbeat pipeline (separate query)
					GbVerified: 0, // populated by traffic verifier (separate query)
					Gs:         uint16(score),
				})
			}
		}
	}

	if len(entries) == 0 {
		return
	}

	log.Printf("[PEAQ] Submitting update_stats for %d nodes", len(entries))
	repoBatchID := sha256.Sum256([]byte(batchDate + "-reputation"))
	_, err = peaqClient.SendUpdateStats(repoBatchID, entries, nil)
	if err != nil {
		log.Printf("[Oracle] On-chain update_stats failed: %v", err)
	}
}
