package models

import (
	"database/sql"
	"exra/db"
	"fmt"
	"log"
	"sort"
	"time"
)

type FeederAssignment struct {
	ID             int64     `json:"id"`
	TargetDeviceID string    `json:"target_device_id"`
	FeederDeviceID string    `json:"feeder_device_id"`
	TargetSubnet   string    `json:"target_subnet"`
	FeederSubnet   string    `json:"feeder_subnet"`
	AssignedAt     time.Time `json:"assigned_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Status         string    `json:"status"`
}

// AssignFeeder selects a candidate node to audit a target node.
// logic: GS > 300 (rs_mult > 0.6), different /24 subnet, stake > 10 EXRA.
func AssignFeeder(targetDeviceID string) (*FeederAssignment, error) {
	var targetIP string
	err := db.DB.QueryRow(`SELECT COALESCE(ip, '') FROM nodes WHERE device_id = $1`, targetDeviceID).Scan(&targetIP)
	if err != nil {
		return nil, err
	}
	targetSubnet := toSubnet24(targetIP)

	// Select a random eligible feeder
	var feeder FeederAssignment
	err = db.DB.QueryRow(`
		SELECT device_id, ip
		FROM nodes
		WHERE device_id != $1
		  AND active = true
		  AND status != 'frozen'
		  AND rs_mult > 0.6
		  AND stake_exra >= 10
		  AND ip NOT LIKE $2
		ORDER BY RANDOM()
		LIMIT 1`,
		targetDeviceID, targetSubnet+"%",
	).Scan(&feeder.FeederDeviceID, &feeder.FeederSubnet)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no eligible feeder found for target %s", targetDeviceID)
		}
		return nil, err
	}

	feeder.TargetDeviceID = targetDeviceID
	feeder.TargetSubnet = targetSubnet
	feeder.FeederSubnet = toSubnet24(feeder.FeederSubnet)

	err = db.DB.QueryRow(`
		INSERT INTO feeder_assignments (target_device_id, feeder_device_id, target_subnet, feeder_subnet, expires_at)
		VALUES ($1, $2, $3, $4, NOW() + INTERVAL '1 hour')
		RETURNING id, assigned_at, expires_at, status`,
		feeder.TargetDeviceID, feeder.FeederDeviceID, feeder.TargetSubnet, feeder.FeederSubnet,
	).Scan(&feeder.ID, &feeder.AssignedAt, &feeder.ExpiresAt, &feeder.Status)

	return &feeder, err
}

// RecordFeederReport persists a feeder's verdict on a target.
func RecordFeederReport(assignmentID int64, feederID, targetID, verdict string, reportedBytes, expectedBytes int64) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Verify and update assignment
	var currentStatus string
	err = tx.QueryRow(`
		SELECT status FROM feeder_assignments 
		WHERE id = $1 AND feeder_device_id = $2 AND status = 'pending' AND expires_at > NOW()
		FOR UPDATE`,
		assignmentID, feederID,
	).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("invalid or expired assignment: %w", err)
	}

	_, err = tx.Exec(`UPDATE feeder_assignments SET status = 'reported' WHERE id = $1`, assignmentID)
	if err != nil {
		return err
	}

	// 2. Insert report
	_, err = tx.Exec(`
		INSERT INTO feeder_reports (assignment_id, feeder_device_id, target_device_id, verdict, reported_bytes, expected_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		assignmentID, feederID, targetID, verdict, reportedBytes, expectedBytes,
	)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// Trigger async evaluation (in real system this might be a background worker)
	go EvaluateFeederConsensus(targetID)
	return nil
}

// EvaluateFeederConsensus performs a 2/3 majority check on a target's recent reports.
func EvaluateFeederConsensus(targetID string) {
	// 1. Fetch recent reports (last 4 hours)
	rows, err := db.DB.Query(`
		SELECT id, feeder_device_id, verdict 
		FROM feeder_reports 
		WHERE target_device_id = $1 AND evaluated_at IS NULL
		  AND created_at > NOW() - INTERVAL '4 hours'`,
		targetID,
	)
	if err != nil {
		log.Printf("[FeederAudit] Failed to fetch reports for %s: %v", targetID, err)
		return
	}
	defer rows.Close()

	type rpt struct {
		id       int64
		feederID string
		verdict  string
	}
	var reports []rpt
	fraudCount := 0
	honestCount := 0

	for rows.Next() {
		var r rpt
		if err := rows.Scan(&r.id, &r.feederID, &r.verdict); err == nil {
			reports = append(reports, r)
		}
	}

	// Sort reports by ID for deterministic processing (important for tests)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].id < reports[j].id
	})

	// 2. Wait for at least 3 reports for a valid consensus
	if len(reports) < 3 {
		return
	}

	fraudCount = 0
	honestCount = 0
	for _, r := range reports {
		if r.verdict == "fraud" {
			fraudCount++
		} else {
			honestCount++
		}
	}

	majorityVerdict := "honest"
	if float64(fraudCount)/float64(len(reports)) >= 0.66 {
		majorityVerdict = "fraud"
	}

	log.Printf("[FeederAudit] Consensus for %s: %d total (%d fraud, %d honest) -> %s", 
		targetID, len(reports), fraudCount, honestCount, majorityVerdict)

	// 2. Apply consequences
	if majorityVerdict == "fraud" {
		_ = FreezeNode(targetID, "feeder_consensus_fraud")
	}

	// 3. Reward/Slash feeders based on their contribution to majority
	for _, r := range reports {
		correct := (r.verdict == majorityVerdict)
		evalStatus := "correct"
		if !correct {
			if majorityVerdict == "fraud" {
				evalStatus = "false_negative"
			} else {
				evalStatus = "false_positive"
			}
			SlashFeeder(r.feederID, 0.05) // -5% stake penalty
		}
		
		db.DB.Exec(`UPDATE feeder_reports SET evaluated_at = NOW(), evaluation_result = $1 WHERE id = $2`, evalStatus, r.id)
		db.DB.Exec(`UPDATE feeder_assignments SET status = 'evaluated' WHERE id = (SELECT assignment_id FROM feeder_reports WHERE id = $1)`, r.id)
	}
}

// SlashFeeder penalizes a feeder for providing an incorrect report (outlier in consensus).
func SlashFeeder(feederID string, percent float64) {
	_, err := db.DB.Exec(`
		UPDATE nodes 
		SET stake_exra = stake_exra * (1.0 - $2),
		    updated_at = NOW()
		WHERE device_id = $1`,
		feederID, percent,
	)
	if err != nil {
		log.Printf("[FeederAudit] Failed to slash feeder %s: %v", feederID, err)
	}
}
