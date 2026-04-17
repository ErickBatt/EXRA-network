package models

import (
	"crypto/sha256"
	"encoding/hex"
	"exra/db"
	"fmt"
	"log"
	"time"

	"context"
)

// StartAttestationWorker запускает фоновую аттестацию состояния Redis на peaq (каждый час).
// Как указано в v2.1 Security Matrix: Oracle Tamper protection.
func StartAttestationWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := runAttestation(); err != nil {
					log.Printf("[Security] Attestation failed: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func runAttestation() error {
	// 1. Собираем Merkle Hash текущего состояния нод (Score + DID)
	// В реальной системе это Merkle Root из Redis ZSET.
	// Упрощенно: хеш от списка топовых нод.
	
	nodes, err := GetActiveNodes("bandwidth")
	if err != nil {
		return err
	}

	h := sha256.New()
	for _, n := range nodes {
		h.Write([]byte(n.DeviceID))
		h.Write([]byte(fmt.Sprintf("%f", n.PricePerGB)))
	}
	stateRoot := hex.EncodeToString(h.Sum(nil))

	// 2. Публикуем в Redis для оракулов
	if globalHub != nil {
		// globalHub.SetStateRoot(stateRoot) // TODO: Implement in hub
	}

	// 3. Записываем в БД лог аттестации
	_, err = db.DB.Exec(`
		INSERT INTO attestation_logs (state_root, node_count, created_at)
		VALUES ($1, $2, NOW())`,
		stateRoot, len(nodes),
	)
	
	log.Printf("[Security] State Root Attestation completed: %s (%d nodes)", stateRoot, len(nodes))
	
	return err
}

// GenerateSessionToken создает JWT-подобный токен для сессии с подписью.
func GenerateSessionToken(buyerID, nodeDID string, estGB float64) string {
	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("%s:%s:%f:%d", buyerID, nodeDID, estGB, timestamp)
	
	// В продакшене используем RSA/Ed25519. Здесь HMAC для POC.
	h := sha256.Sum256([]byte(payload + "PROXY_SECRET_TODO"))
	sig := hex.EncodeToString(h[:])
	
	return fmt.Sprintf("%x.%s", sig, payload)
}
