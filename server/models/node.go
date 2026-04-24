package models

import (
	"database/sql"
	"encoding/json"
	"exra/db"
	"fmt"
	"log"
	"strings"
	"time"
)

const NodeEarnUSDPerGB = 0.30

type Node struct {
	ID            string    `json:"id"`
	DeviceID      string    `json:"device_id"`
	IP            string    `json:"ip"`
	Address       string    `json:"address"`
	Port          int       `json:"port"`
	Country       string    `json:"country"`
	DeviceType    string    `json:"device_type"`
	DeviceTier    string    `json:"device_tier"`
	IsResidential bool      `json:"is_residential"`
	ASNOrg        string    `json:"asn_org"`
	Status        string    `json:"status"`
	TrafficBytes  int64     `json:"traffic_bytes"`
	BandwidthMbps int       `json:"bandwidth_mbps"`
	CPUModel      string    `json:"cpu_model" db:"cpu_model"`
	CPUCores      int       `json:"cpu_cores" db:"cpu_cores"`
	VRAMMB        int       `json:"vram_mb" db:"vram_mb"`
	RAMMB         int       `json:"ram_mb" db:"ram_mb"`
	DID           string    `json:"did" db:"did"`
	IdentityTier  string    `json:"identity_tier" db:"identity_tier"`
	PublicKey     string    `json:"public_key"`
	Active        bool      `json:"active" db:"active"`
	PricePerGB    float64   `json:"price_per_gb"`
	RSScore       float64   `json:"rs_score"` // Internal reputation score
	RSTier        string    `json:"rs_tier"`  // A, B, C
	Uptime        float64   `json:"uptime_pct"`
	AutoPrice     bool      `json:"auto_price"`
	LastSeen      time.Time `json:"last_seen"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time `json:"created_at"`
}

type PublicNode struct {
	ID            string    `json:"id"`
	DeviceID      string    `json:"device_id"`
	IP            string    `json:"ip"`
	Address       string    `json:"address"`
	Port          int       `json:"port"`
	Country       string    `json:"country"`
	DeviceType    string    `json:"device_type"`
	DeviceTier    string    `json:"device_tier"`
	IsResidential bool      `json:"is_residential"`
	ASNOrg        string    `json:"asn_org"`
	Status        string    `json:"status"`
	BandwidthMbps int       `json:"bandwidth_mbps"`
	CPUModel      string    `json:"cpu_model"`
	CPUCores      int       `json:"cpu_cores"`
	VRAMMB        int       `json:"vram_mb"`
	RAMMB         int       `json:"ram_mb"`
	DID           string    `json:"did"`
	IdentityTier  string    `json:"identity_tier"`
	RSScore       float64   `json:"rs_score"` // Internal reputation score
	RSTier        string    `json:"rs_tier"`  // A, B, C
	Uptime        float64   `json:"uptime_pct"`
	PricePerGB    float64   `json:"price_per_gb"`
	LastSeen      time.Time `json:"last_seen"`
}

type NodeFilter struct {
	Country      string
	MinVRAM      int
	MaxPrice     float64
	Tier         string
	IdentityTier string
}

type RewardPolicy struct {
	BaseRateUSDPerGB float64 `json:"base_rate_usd_per_gb"`
	TierMultiplier   float64 `json:"tier_multiplier"`
	QualityFactor    float64 `json:"quality_factor"`
	ReasonCode       string  `json:"reason_code"`
	Quarantined      bool    `json:"quarantined"`
}

type NodeStats struct {
	OnlineNodes       int64 `json:"online_nodes"`
	TotalNodes        int64 `json:"total_nodes"`
	TotalTrafficBytes int64 `json:"total_traffic_bytes"`
}

func RegisterNode(address string, port int, country string, bandwidthMbps int) (*Node, error) {
	node := &Node{}
	err := db.DB.QueryRow(
		`INSERT INTO nodes (address, port, country, bandwidth_mbps)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, COALESCE(device_id, ''), COALESCE(ip, ''), address, port, country, COALESCE(device_type, ''), COALESCE(device_tier, 'network'), COALESCE(is_residential, true), COALESCE(asn_org, ''), COALESCE(status, 'online'), COALESCE(traffic_bytes, 0), bandwidth_mbps, active, COALESCE(price_per_gb, 1.50), COALESCE(auto_price, true), COALESCE(last_seen, NOW()), last_heartbeat, created_at`,
		address, port, country, bandwidthMbps,
	).Scan(&node.ID, &node.DeviceID, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps, &node.Active, &node.PricePerGB, &node.AutoPrice, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt)
	return node, err
}

func Heartbeat(nodeID string) error {
	_, err := db.DB.Exec(
		`UPDATE nodes SET last_heartbeat = NOW(), last_seen = NOW(), active = true, status = 'online' WHERE id = $1`,
		nodeID,
	)
	return err
}

func UpsertWSNode(deviceID, publicKey, ip, country, deviceType, tier, asnOrg string, isResidential bool, cpuModel string, cpuCores, vramMB, ramMB int, autoPrice bool, pricePerGB float64, did string) (*Node, error) {
	node := &Node{}
	err := db.DB.QueryRow(`
		INSERT INTO nodes (device_id, public_key, ip, country, device_type, device_tier, is_residential, asn_org, cpu_model, cpu_cores, vram_mb, ram_mb, auto_price, price_per_gb, did, status, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 'online', true)
		ON CONFLICT (device_id) DO UPDATE SET
		   ip = EXCLUDED.ip,
		   country = EXCLUDED.country,
		   device_type = CASE WHEN EXCLUDED.device_type != '' THEN EXCLUDED.device_type ELSE nodes.device_type END,
		   cpu_model = CASE WHEN EXCLUDED.cpu_model != '' THEN EXCLUDED.cpu_model ELSE nodes.cpu_model END,
		   cpu_cores = CASE WHEN EXCLUDED.cpu_cores > 0 THEN EXCLUDED.cpu_cores ELSE nodes.cpu_cores END,
		   vram_mb = CASE WHEN EXCLUDED.vram_mb > 0 THEN EXCLUDED.vram_mb ELSE nodes.vram_mb END,
		   ram_mb = CASE WHEN EXCLUDED.ram_mb > 0 THEN EXCLUDED.ram_mb ELSE nodes.ram_mb END,
		   did = CASE WHEN EXCLUDED.did != '' THEN EXCLUDED.did ELSE nodes.did END,
		   price_per_gb = CASE WHEN EXCLUDED.auto_price = false AND EXCLUDED.price_per_gb > 0 THEN EXCLUDED.price_per_gb ELSE nodes.price_per_gb END,
		   auto_price = EXCLUDED.auto_price,
		   status = 'online',
		   active = true,
		   last_seen = NOW(),
		   last_heartbeat = NOW()
		 RETURNING id, COALESCE(device_id, ''), COALESCE(public_key, ''), COALESCE(ip, ''), address, port, country, COALESCE(device_type, ''), COALESCE(device_tier, 'network'), COALESCE(is_residential, true), COALESCE(asn_org, ''), COALESCE(status, 'online'), COALESCE(traffic_bytes, 0), bandwidth_mbps, COALESCE(cpu_model,''), COALESCE(cpu_cores,0), COALESCE(vram_mb,0), COALESCE(ram_mb,0), COALESCE(did, ''), COALESCE(identity_tier, 'anon'), active, COALESCE(price_per_gb, 1.50), COALESCE(auto_price, true), COALESCE(last_seen, NOW()), last_heartbeat, created_at`,
		deviceID, publicKey, ip, country, deviceType, tier, isResidential, asnOrg, cpuModel, cpuCores, vramMB, ramMB, autoPrice, pricePerGB, did,
	).Scan(&node.ID, &node.DeviceID, &node.PublicKey, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps,
		&node.CPUModel, &node.CPUCores, &node.VRAMMB, &node.RAMMB, &node.DID, &node.IdentityTier,
		&node.Active, &node.PricePerGB, &node.AutoPrice, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt)
	return node, err
}

func SetNodeOfflineByDeviceID(deviceID string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(
		`UPDATE nodes SET status = 'offline', active = false, last_seen = NOW() WHERE device_id = $1`,
		deviceID,
	); err != nil {
		return err
	}
	// Fix #6: auto-pause marketplace listings so buyers don't see stale entries
	// for devices that are offline. The worker must manually resume via the TMA.
	if _, err = tx.Exec(
		`UPDATE worker_listings SET status = 'paused', updated_at = NOW()
		 WHERE device_id = $1 AND status = 'active'`,
		deviceID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func AddNodeTrafficByDeviceID(deviceID string, bytes int64) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE nodes
		 SET traffic_bytes = traffic_bytes + $1, last_seen = NOW(), status = 'online', active = true
		 WHERE device_id = $2`,
		bytes, deviceID,
	)
	if err != nil {
		return err
	}
	var tier, asnOrg, nodeIP string
	var isResidential bool
	if err := tx.QueryRow(`SELECT COALESCE(device_tier,'network'), COALESCE(asn_org,''), COALESCE(is_residential,true), COALESCE(ip,'') FROM nodes WHERE device_id = $1`, deviceID).Scan(&tier, &asnOrg, &isResidential, &nodeIP); err != nil {
		return err
	}
	policy := calculateRewardPolicy(tier, isResidential, asnOrg, bytes)
	var recentEvents int64
	_ = tx.QueryRow(
		`SELECT COUNT(*)
		 FROM reward_events
		 WHERE device_id = $1 AND created_at > NOW() - INTERVAL '1 minute'`,
		deviceID,
	).Scan(&recentEvents)
	if recentEvents > 60 {
		policy.QualityFactor = 0
		policy.ReasonCode = "velocity_limit"
		policy.Quarantined = true
	}
	// Sybil check: /24 subnet density penalty
	if nodeIP != "" {
		penalty := SybilSubnetPenalty(tx, nodeIP)
		if penalty < 1.0 {
			policy.QualityFactor *= penalty
			if penalty <= 0.1 {
				policy.ReasonCode = "sybil_farm_detected"
				policy.Quarantined = true
			} else {
				policy.ReasonCode = "sybil_cluster_penalty"
				// Not fully quarantined — still earns at reduced rate
			}
		}
	}
	earnedUSD := float64(bytes) / (1024 * 1024 * 1024) * policy.BaseRateUSDPerGB * policy.TierMultiplier * policy.QualityFactor
	if policy.Quarantined {
		earnedUSD = 0
	}
	_, err = tx.Exec(
		`INSERT INTO node_earnings (device_id, bytes, earned_usd)
		 VALUES ($1, $2, $3)`,
		deviceID, bytes, earnedUSD,
	)
	if err != nil {
		return err
	}
	snapshot, _ := json.Marshal(policy)
	var rewardEventID int64
	if err := tx.QueryRow(
		`INSERT INTO reward_events (device_id, bytes, base_rate_usd_per_gb, tier_multiplier, quality_factor, earned_usd, reason_code, policy_snapshot, quarantined)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)
		 RETURNING id`,
		deviceID, bytes, policy.BaseRateUSDPerGB, policy.TierMultiplier, policy.QualityFactor, earnedUSD, policy.ReasonCode, string(snapshot), policy.Quarantined,
	).Scan(&rewardEventID); err != nil {
		return err
	}
	if !policy.Quarantined && earnedUSD > 0 {
		if _, err := tx.Exec(
			`INSERT INTO oracle_mint_queue (reward_event_id, device_id, amount_exra, status)
			 VALUES ($1, $2, $3, 'pending')`,
			rewardEventID, deviceID, earnedUSD,
		); err != nil {
			return err
		}
	}
	// E3 cross-check: accumulate worker-reported bytes on the active session
	// for this node. FinalizeSession uses MAX(gateway_bytes, worker_bytes) so
	// an underreporting Gateway cannot leave workers unpaid.
	if _, err := tx.Exec(
		`UPDATE sessions s
		 SET worker_bytes_reported = worker_bytes_reported + $1
		 FROM nodes n
		 WHERE n.id = s.node_id
		   AND n.device_id = $2
		   AND s.active = true`,
		bytes, deviceID,
	); err != nil {
		log.Printf("[E3] failed to update worker_bytes_reported device=%s: %v", deviceID, err)
	}
	return tx.Commit()
}

func calculateRewardPolicy(tier string, isResidential bool, asnOrg string, bytes int64) RewardPolicy {
	p := RewardPolicy{
		BaseRateUSDPerGB: NodeEarnUSDPerGB,
		TierMultiplier:   1.0,
		QualityFactor:    1.0,
		ReasonCode:       "ok",
		Quarantined:      false,
	}
	if tier == "compute" {
		p.TierMultiplier = 3.0
	}
	asn := strings.ToLower(asnOrg)
	if !isResidential || strings.Contains(asn, "aws") || strings.Contains(asn, "digitalocean") || strings.Contains(asn, "hetzner") {
		p.QualityFactor = 0
		p.ReasonCode = "datacenter_or_non_residential"
		p.Quarantined = true
	}
	if bytes <= 0 {
		p.QualityFactor = 0
		p.ReasonCode = "invalid_bytes"
		p.Quarantined = true
	}
	return p
}

func GetActiveNodeByCountry(country string) (*Node, error) {
	node := &Node{}
	err := db.DB.QueryRow(
		`SELECT n.id, COALESCE(n.device_id, ''), COALESCE(n.ip, ''), n.address, n.port, n.country, 
		        COALESCE(n.device_type, ''), COALESCE(n.device_tier,'network'), COALESCE(n.is_residential, true), 
		        COALESCE(n.asn_org, ''), COALESCE(n.status, 'online'), COALESCE(n.traffic_bytes, 0), n.bandwidth_mbps, 
		        COALESCE(n.cpu_model,''), COALESCE(n.cpu_cores,0), COALESCE(n.vram_mb,0), COALESCE(n.ram_mb,0),
		        n.active, COALESCE(n.price_per_gb, 1.50), COALESCE(n.auto_price, true), COALESCE(n.last_seen, NOW()), n.last_heartbeat, n.created_at
		 FROM nodes n
		 LEFT JOIN (
		    SELECT node_id, count(*) as active_count
		    FROM sessions
		    WHERE ended_at IS NULL
		    GROUP BY node_id
		 ) s ON s.node_id = n.id
		 WHERE n.active = true
		   AND n.status = 'online'
		   AND n.last_heartbeat > NOW() - INTERVAL '2 minutes'
		   AND n.country = $1
		 ORDER BY (n.bandwidth_mbps::float / (COALESCE(s.active_count, 0) + 1)) DESC, n.last_heartbeat DESC
		 LIMIT 1`,
		country,
	).Scan(&node.ID, &node.DeviceID, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps,
		&node.CPUModel, &node.CPUCores, &node.VRAMMB, &node.RAMMB,
		&node.Active, &node.PricePerGB, &node.AutoPrice, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func GetActiveNodeByID(nodeID string) (*Node, error) {
	node := &Node{}
	err := db.DB.QueryRow(
		`SELECT id, COALESCE(device_id, ''), COALESCE(ip, ''), address, port, country,
		        COALESCE(device_type, ''), COALESCE(device_tier, 'network'), COALESCE(is_residential, true),
		        COALESCE(asn_org, ''), COALESCE(status, 'online'), COALESCE(traffic_bytes, 0), bandwidth_mbps,
		        COALESCE(cpu_model,''), COALESCE(cpu_cores,0), COALESCE(vram_mb,0), COALESCE(ram_mb,0),
		        active, COALESCE(price_per_gb, 1.50), COALESCE(auto_price, true), COALESCE(last_seen, NOW()), last_heartbeat, created_at
		 FROM nodes
		 WHERE id = $1
		   AND active = true
		   AND status = 'online'
		   AND last_heartbeat > NOW() - INTERVAL '2 minutes'
		 LIMIT 1`,
		nodeID,
	).Scan(&node.ID, &node.DeviceID, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps,
		&node.CPUModel, &node.CPUCores, &node.VRAMMB, &node.RAMMB,
		&node.Active, &node.PricePerGB, &node.AutoPrice, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// GetActiveNodes returns all online nodes sorted by a specific key.

func GetActiveNodes(sortBy string) ([]Node, error) {
	return GetActiveNodesWithFilters(sortBy, NodeFilter{})
}

func GetActiveNodesWithFilters(sortBy string, filter NodeFilter) ([]Node, error) {
	var orderBy string
	switch sortBy {
	case "price_asc":
		orderBy = "COALESCE(price_per_gb, 1.50) ASC, created_at DESC"
	case "price_desc":
		orderBy = "COALESCE(price_per_gb, 1.50) DESC, created_at DESC"
	case "bandwidth":
		orderBy = "bandwidth_mbps DESC, created_at DESC"
	default:
		orderBy = "created_at DESC"
	}

	where := "active = true AND status = 'online' AND last_heartbeat > NOW() - INTERVAL '2 minutes'"
	args := []any{}
	argIdx := 1

	if filter.Country != "" {
		where += fmt.Sprintf(" AND country = $%d", argIdx)
		args = append(args, filter.Country)
		argIdx++
	}
	if filter.MinVRAM > 0 {
		where += fmt.Sprintf(" AND vram_mb >= $%d", argIdx)
		args = append(args, filter.MinVRAM)
		argIdx++
	}
	if filter.MaxPrice > 0 {
		where += fmt.Sprintf(" AND price_per_gb <= $%d", argIdx)
		args = append(args, filter.MaxPrice)
		argIdx++
	}
	if filter.Tier != "" {
		where += fmt.Sprintf(" AND device_tier = $%d", argIdx)
		args = append(args, filter.Tier)
		argIdx++
	}
	if filter.IdentityTier != "" {
		where += fmt.Sprintf(" AND identity_tier = $%d", argIdx)
		args = append(args, filter.IdentityTier)
		argIdx++
	}

	query := fmt.Sprintf(`SELECT id, COALESCE(device_id, ''), COALESCE(ip, ''), address, port, country,
		        COALESCE(device_type, ''), COALESCE(device_tier, 'network'), COALESCE(is_residential, true),
		        COALESCE(asn_org, ''), COALESCE(status, 'online'), COALESCE(traffic_bytes, 0), bandwidth_mbps,
		        COALESCE(cpu_model,''), COALESCE(cpu_cores,0), COALESCE(vram_mb,0), COALESCE(ram_mb,0),
		        active, COALESCE(price_per_gb, 1.50), COALESCE(auto_price, true), COALESCE(last_seen, NOW()), last_heartbeat, created_at,
		        COALESCE(did, ''), COALESCE(identity_tier, 'anon')
		 FROM nodes
		 WHERE %s
		 ORDER BY %s`, where, orderBy)

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.DeviceID, &n.IP, &n.Address, &n.Port, &n.Country,
			&n.DeviceType, &n.DeviceTier, &n.IsResidential, &n.ASNOrg, &n.Status, &n.TrafficBytes, &n.BandwidthMbps,
			&n.CPUModel, &n.CPUCores, &n.VRAMMB, &n.RAMMB,
			&n.Active, &n.PricePerGB, &n.AutoPrice, &n.LastSeen, &n.LastHeartbeat, &n.CreatedAt,
			&n.DID, &n.IdentityTier); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func GetBestNode() (*Node, error) {
	node := &Node{}
	err := db.DB.QueryRow(
		`SELECT n.id, COALESCE(n.device_id, ''), COALESCE(n.ip, ''), n.address, n.port, n.country, COALESCE(n.device_type, ''), COALESCE(n.device_tier,'network'), COALESCE(n.is_residential, true), COALESCE(n.asn_org, ''), COALESCE(n.status, 'online'), COALESCE(n.traffic_bytes, 0), n.bandwidth_mbps, n.active, COALESCE(n.price_per_gb, 1.50), COALESCE(n.auto_price, true), COALESCE(n.last_seen, NOW()), n.last_heartbeat, n.created_at
		 FROM nodes n
		 LEFT JOIN (
		    SELECT node_id, count(*) as active_count
		    FROM sessions
		    WHERE ended_at IS NULL
		    GROUP BY node_id
		 ) s ON s.node_id = n.id
		 WHERE n.active = true AND n.status = 'online' AND n.last_heartbeat > NOW() - INTERVAL '2 minutes'
		 ORDER BY (n.bandwidth_mbps::float / (COALESCE(s.active_count, 0) + 1)) DESC
		 LIMIT 1`,
	).Scan(&node.ID, &node.DeviceID, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps, &node.Active, &node.PricePerGB, &node.AutoPrice, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func GetNodeStats() (*NodeStats, error) {
	stats := &NodeStats{}
	var online sql.NullInt64
	var total sql.NullInt64
	var traffic sql.NullInt64
	err := db.DB.QueryRow(
		`SELECT
		   COUNT(*) FILTER (WHERE active = true AND status = 'online' AND last_heartbeat > NOW() - INTERVAL '2 minutes') AS online_nodes,
		   COUNT(*) AS total_nodes,
		   COALESCE(SUM(traffic_bytes), 0) AS total_traffic
		 FROM nodes`,
	).Scan(&online, &total, &traffic)
	if err != nil {
		return nil, err
	}
	stats.OnlineNodes = online.Int64
	stats.TotalNodes = total.Int64
	stats.TotalTrafficBytes = traffic.Int64
	return stats, nil
}

func GetMarketAvgPrice(country string) (float64, error) {
	var avg sql.NullFloat64
	// Attempt to get from aggregated table first
	err := db.DB.QueryRow(
		`SELECT avg_price FROM market_avg_price WHERE country = $1 AND updated_at > NOW() - INTERVAL '1 hour'`,
		country,
	).Scan(&avg)
	
	if err == nil && avg.Valid {
		return avg.Float64, nil
	}

	// Fallback: calculate real average price from currently active nodes in this country
	err = db.DB.QueryRow(
		`SELECT AVG(price_per_gb) 
		 FROM nodes 
		 WHERE country = $1 
		   AND active = true 
		   AND last_heartbeat > NOW() - INTERVAL '5 minutes'`,
		country,
	).Scan(&avg)

	if err != nil || !avg.Valid {
		return 1.50, nil // Hard fallback
	}
	return avg.Float64, nil
}

// UpgradeNodeToPeak triggers the on-chain (and internal) flip of identity tier.
// 100 EXRA is deducted from the node's batched credits or directly from account.
func UpgradeNodeToPeak(did string, stakeAmount float64) error {
	if stakeAmount < 100 {
		return fmt.Errorf("insufficient stake: 100 EXRA required for Peak tier")
	}

	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Verify existence and current tier
	var currentTier string
	err = tx.QueryRow(`SELECT identity_tier FROM nodes WHERE did = $1 FOR UPDATE`, did).Scan(&currentTier)
	if err != nil {
		return fmt.Errorf("did not found: %v", err)
	}
	if currentTier == "peak" {
		return fmt.Errorf("already at Peak tier")
	}

	// 2. Perform the flip (In production, this also sends a peaq extrinsic)
	_, err = tx.Exec(`
		UPDATE nodes
		SET identity_tier = 'peak',
		    stake_exra = stake_exra + $2,
		    timelock_hours = 0,
		    rs_mult = 2.0, 
		    updated_at = NOW()
		WHERE did = $1`,
		did, stakeAmount,
	)
	if err != nil {
		return err
	}

	// 3. Log the upgrade event
	_, err = tx.Exec(`
		INSERT INTO identity_events (did, event_type, metadata)
		VALUES ($1, 'upgrade_to_peak', $2)`,
		did, `{"stake": 100, "reason": "staking_completed"}`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func GetNodePublicKeyByDID(did string, dest *string) error {
	return db.DB.QueryRow(`SELECT public_key FROM nodes WHERE did = $1`, did).Scan(dest)
}

func GetNodePublicKey(deviceID string, dest *string) error {
	return db.DB.QueryRow(`SELECT public_key FROM nodes WHERE device_id = $1`, deviceID).Scan(dest)
}

// GetNodeAuthByDeviceID fetches the DID and public key for a device in a
// single round-trip. Used by TunnelHandler (AUDIT §1 G1) to verify that an
// incoming tunnel request is signed by the node the matcher selected.
func GetNodeAuthByDeviceID(deviceID string) (pubKey, did string, err error) {
	err = db.DB.QueryRow(
		`SELECT COALESCE(public_key, ''), COALESCE(did, '') FROM nodes WHERE device_id = $1`,
		deviceID,
	).Scan(&pubKey, &did)
	return
}
