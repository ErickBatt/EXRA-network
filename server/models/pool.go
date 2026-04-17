package models

import (
	"database/sql"
	"exra/db"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Pool is a reputation-based node guild with tiered treasury fee discounts.
type Pool struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	OwnerDeviceID  string    `json:"owner_device_id"`
	Description    string    `json:"description"`
	NodeCount      int       `json:"node_count"`
	AvgUptimePct   float64   `json:"avg_uptime_pct"`
	Tier           string    `json:"tier"`             // solo | silver | gold | platinum
	TreasuryFeePct float64   `json:"treasury_fee_pct"` // 10–30%
	TotalEarnedExra float64  `json:"total_earned_exra"`
	IsPublic       bool      `json:"is_public"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PoolMember is a node's membership record.
type PoolMember struct {
	PoolID    string    `json:"pool_id"`
	DeviceID  string    `json:"device_id"`
	JoinedAt  time.Time `json:"joined_at"`
}

// PoolTierFee returns the treasury fee percentage (0.0–1.0 fraction) for a pool.
//   Platinum: 500+ nodes, 98%+ uptime → 10%
//   Gold:     100+ nodes, 95%+ uptime → 15%
//   Silver:   10–99 nodes             → 20%
//   Solo:     < 10 nodes              → 30% (default)
func PoolTierFee(nodeCount int, avgUptimePct float64) (tier string, feePct float64) {
	switch {
	case nodeCount >= 500 && avgUptimePct >= 98:
		return "platinum", 0.10
	case nodeCount >= 100 && avgUptimePct >= 95:
		return "gold", 0.15
	case nodeCount >= 10:
		return "silver", 0.20
	default:
		return "solo", 0.30
	}
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]`)

func toSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// CreatePool creates a new pool and makes the owner the first member.
func CreatePool(ownerDeviceID, name, description string, isPublic bool) (*Pool, error) {
	slug := toSlug(name)
	if slug == "" {
		return nil, fmt.Errorf("pool name produces an empty slug")
	}
	pool := &Pool{}
	err := db.DB.QueryRow(`
		INSERT INTO pools (name, slug, owner_device_id, description, is_public,
		                   node_count, avg_uptime_pct, tier, treasury_fee_pct)
		VALUES ($1, $2, $3, $4, $5, 1, 0, 'solo', 30)
		RETURNING id, name, slug, owner_device_id, description, node_count,
		          avg_uptime_pct, tier, treasury_fee_pct, total_earned_exra,
		          is_public, created_at, updated_at`,
		name, slug, ownerDeviceID, description, isPublic,
	).Scan(
		&pool.ID, &pool.Name, &pool.Slug, &pool.OwnerDeviceID, &pool.Description,
		&pool.NodeCount, &pool.AvgUptimePct, &pool.Tier, &pool.TreasuryFeePct,
		&pool.TotalEarnedExra, &pool.IsPublic, &pool.CreatedAt, &pool.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	// Add owner as first member.
	_, _ = db.DB.Exec(`
		INSERT INTO pool_members (pool_id, device_id) VALUES ($1, $2)
		ON CONFLICT (device_id) DO NOTHING`,
		pool.ID, ownerDeviceID,
	)
	return pool, nil
}

// JoinPool adds a node to a pool (leaves any previous pool automatically).
func JoinPool(poolID, deviceID string) error {
	_, err := db.DB.Exec(`
		INSERT INTO pool_members (pool_id, device_id)
		VALUES ($1, $2)
		ON CONFLICT (device_id) DO UPDATE SET pool_id = $1, joined_at = NOW()`,
		poolID, deviceID,
	)
	if err != nil {
		return err
	}
	return refreshPoolStats(poolID)
}

// LeavePool removes a node from its current pool.
func LeavePool(deviceID string) error {
	var poolID string
	if err := db.DB.QueryRow(
		`DELETE FROM pool_members WHERE device_id = $1 RETURNING pool_id`,
		deviceID,
	).Scan(&poolID); err != nil {
		return err // not in a pool
	}
	return refreshPoolStats(poolID)
}

// refreshPoolStats recomputes node_count, avg_uptime_pct, tier, and treasury_fee_pct.
func refreshPoolStats(poolID string) error {
	var nodeCount int
	var avgUptime float64
	_ = db.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(AVG(n.uptime_pct), 0)
		FROM pool_members pm
		JOIN nodes n ON n.device_id = pm.device_id
		WHERE pm.pool_id = $1`,
		poolID,
	).Scan(&nodeCount, &avgUptime)

	tier, feePct := PoolTierFee(nodeCount, avgUptime)
	_, err := db.DB.Exec(`
		UPDATE pools
		SET node_count = $1, avg_uptime_pct = $2, tier = $3,
		    treasury_fee_pct = $4, updated_at = NOW()
		WHERE id = $5`,
		nodeCount, avgUptime, tier, feePct*100, poolID,
	)
	return err
}

// GetPoolByDevice returns the pool a device belongs to, or nil if not in any pool.
func GetPoolByDevice(tx *sql.Tx, deviceID string) (*Pool, error) {
	pool := &Pool{}
	query := `
		SELECT p.id, p.name, p.slug, p.owner_device_id, p.description,
		       p.node_count, p.avg_uptime_pct, p.tier, p.treasury_fee_pct,
		       p.total_earned_exra, p.is_public, p.created_at, p.updated_at
		FROM pools p
		JOIN pool_members pm ON pm.pool_id = p.id
		WHERE pm.device_id = $1`

	var err error
	if tx != nil {
		err = tx.QueryRow(query, deviceID).Scan(
			&pool.ID, &pool.Name, &pool.Slug, &pool.OwnerDeviceID, &pool.Description,
			&pool.NodeCount, &pool.AvgUptimePct, &pool.Tier, &pool.TreasuryFeePct,
			&pool.TotalEarnedExra, &pool.IsPublic, &pool.CreatedAt, &pool.UpdatedAt,
		)
	} else {
		err = db.DB.QueryRow(query, deviceID).Scan(
			&pool.ID, &pool.Name, &pool.Slug, &pool.OwnerDeviceID, &pool.Description,
			&pool.NodeCount, &pool.AvgUptimePct, &pool.Tier, &pool.TreasuryFeePct,
			&pool.TotalEarnedExra, &pool.IsPublic, &pool.CreatedAt, &pool.UpdatedAt,
		)
	}

	if err != nil {
		return nil, nil // not in a pool — not an error
	}
	return pool, nil
}

// GetPool returns a pool by ID.
func GetPool(poolID string) (*Pool, error) {
	pool := &Pool{}
	err := db.DB.QueryRow(`
		SELECT id, name, slug, owner_device_id, description,
		       node_count, avg_uptime_pct, tier, treasury_fee_pct,
		       total_earned_exra, is_public, created_at, updated_at
		FROM pools WHERE id = $1`,
		poolID,
	).Scan(
		&pool.ID, &pool.Name, &pool.Slug, &pool.OwnerDeviceID, &pool.Description,
		&pool.NodeCount, &pool.AvgUptimePct, &pool.Tier, &pool.TreasuryFeePct,
		&pool.TotalEarnedExra, &pool.IsPublic, &pool.CreatedAt, &pool.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

// ListPools returns public pools ordered by node count.
func ListPools(limit int) ([]Pool, error) {
	rows, err := db.DB.Query(`
		SELECT id, name, slug, owner_device_id, description,
		       node_count, avg_uptime_pct, tier, treasury_fee_pct,
		       total_earned_exra, is_public, created_at, updated_at
		FROM pools
		WHERE is_public = true
		ORDER BY node_count DESC, created_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pool
	for rows.Next() {
		var p Pool
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.OwnerDeviceID, &p.Description,
			&p.NodeCount, &p.AvgUptimePct, &p.Tier, &p.TreasuryFeePct,
			&p.TotalEarnedExra, &p.IsPublic, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// PoolTreasuryFeeForDevice returns the treasury fee fraction (0.00–0.25) for a device
// based on its Identity Tier (peaq DePIN v2.0).
//   Anon: 25% tax
//   Peak: 0% tax (staking required)
func PoolTreasuryFeeForDevice(tx *sql.Tx, deviceID string) float64 {
	var tier string
	query := `SELECT identity_tier FROM nodes WHERE device_id = $1`
	var err error
	if tx != nil {
		err = tx.QueryRow(query, deviceID).Scan(&tier)
	} else {
		err = db.DB.QueryRow(query, deviceID).Scan(&tier)
	}

	if err != nil {
		return 0.25 // Default to anon tax if node not found
	}

	if tier == "peak" {
		return 0.0
	}

	// Default: anon tier pays 25% tax to treasury.
	return 0.25
}
