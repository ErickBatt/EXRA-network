package models

import (
	"database/sql"
	"exra/db"
	"fmt"
	"log"
)

// AddStrike добавляет "удар" ноде. Если их > 3 за 24ч — срабатывает слэшинг.
func AddStrike(nodeDID string, reason string) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Записываем страйк в лог
	_, err = tx.Exec(`
		INSERT INTO node_strikes (device_id, reason, created_at)
		VALUES ($1, $2, NOW())`,
		nodeDID, reason,
	)
	if err != nil {
		return err
	}

	// 2. Считаем страйки за последние 24 часа
	var count int
	err = tx.QueryRow(`
		SELECT COUNT(*) FROM node_strikes
		WHERE device_id = $1 AND created_at > NOW() - INTERVAL '24 hours'`,
		nodeDID,
	).Scan(&count)
	if err != nil {
		return err
	}

	// 3. Если страйков > 3 — применяем слэшинг
	if count >= 3 {
		log.Printf("[Slashing] Node %s exceeded 3 strikes. Triggering slash.", nodeDID)
		if err := triggerSlashing(tx, nodeDID); err != nil {
			return fmt.Errorf("failed to trigger slashing: %w", err)
		}
	}

	// 4. Circuit Breaker: Check for network-wide anomaly
	if globalHub != nil {
		rate, err := CheckNetworkAnomaly()
		if err == nil && rate > 0.20 {
			if !globalHub.IsGlobalPause() {
				log.Printf("[CircuitBreaker] Network anomaly detected! Rate: %.2f. Activating Global Pause.", rate)
				globalHub.SetGlobalPause(true)
			}
		}
	}

	return tx.Commit()
}

func triggerSlashing(tx *sql.Tx, nodeDID string) error {
	// В v2.1: Списание 10% стейка и -100 к Reputation Score (RS)
	// RS в нашей системе завязан на rs_mult. Режем rs_mult на 0.1 (эквивалент -100 GS)
	
	_, err := tx.Exec(`
		UPDATE nodes
		SET rs_mult = GREATEST(rs_mult - 0.2, 0.5),
		    stake_exra = stake_exra * 0.9,
		    status = 'quarantined',
		    updated_at = NOW()
		WHERE device_id = $1`,
		nodeDID,
	)
	if err != nil {
		return err
	}

	// TODO: Здесь должен быть вызов peaq pallet для on-chain слэшинга стейка
	log.Printf("[Slashing] Executed: Node %s -10%% stake, -0.2 rs_mult", nodeDID)
	
	return nil
}

// CheckNetworkAnomaly проверяет уровень аномалий в сети (>20% страйков).
func CheckNetworkAnomaly() (float64, error) {
	var rate float64
	err := db.DB.QueryRow(`
		SELECT (COUNT(DISTINCT device_id) FILTER (WHERE created_at > NOW() - INTERVAL '1 hour'))::float / 
		       NULLIF((SELECT COUNT(*) FROM nodes WHERE active = true), 0)
		FROM node_strikes
		WHERE created_at > NOW() - INTERVAL '1 hour'`).Scan(&rate)
	return rate, err
}
