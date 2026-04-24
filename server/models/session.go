package models

import (
	"database/sql"
	"errors"
	"exra/db"
	"log"
	"time"
)

var ErrSessionNotFound = errors.New("session not found")
var ErrInsufficientBuyerBalance = errors.New("insufficient buyer balance for final charge")

type Session struct {
	ID                   string     `json:"id"`
	BuyerID              string     `json:"buyer_id"`
	NodeID               string     `json:"node_id"`
	OfferID              *string    `json:"offer_id,omitempty"`
	StartedAt            time.Time  `json:"started_at"`
	EndedAt              *time.Time `json:"ended_at,omitempty"`
	BytesUsed            int64      `json:"bytes_used"`
	WorkerBytesReported  int64      `json:"worker_bytes_reported"`
	CostUSD              float64    `json:"cost_usd"`
	LockedPricePerGB     float64    `json:"locked_price_per_gb"`
	Active               bool       `json:"active"`
	Billed               bool       `json:"billed"`
}

func CreateSession(buyerID, nodeID string) (*Session, error) {
	session := &Session{}
	var nodePrice float64
	var autoPrice bool
	var nodeCountry string
	if err := db.DB.QueryRow(`SELECT COALESCE(price_per_gb, 1.50), COALESCE(auto_price, true), COALESCE(country, '') FROM nodes WHERE id = $1`, nodeID).Scan(&nodePrice, &autoPrice, &nodeCountry); err != nil {
		return nil, err
	}
	lockedPrice := nodePrice
	if autoPrice {
		if avgPrice, err := GetMarketAvgPrice(nodeCountry); err == nil {
			lockedPrice = avgPrice
		}
	}
	err := db.DB.QueryRow(
		`INSERT INTO sessions (buyer_id, node_id, locked_price_per_gb)
		 VALUES ($1, $2, $3)
		 RETURNING id, buyer_id, node_id, offer_id, started_at, bytes_used, worker_bytes_reported, cost_usd, locked_price_per_gb, active, billed`,
		buyerID, nodeID, lockedPrice,
	).Scan(&session.ID, &session.BuyerID, &session.NodeID, &session.OfferID,
		&session.StartedAt, &session.BytesUsed, &session.WorkerBytesReported, &session.CostUSD, &session.LockedPricePerGB, &session.Active, &session.Billed)
	return session, err
}

func FinalizeSession(sessionID, buyerID string, additionalBytes int64, _ float64) (*Session, bool, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	session := &Session{}
	err = tx.QueryRow(
		`SELECT id, buyer_id, node_id, offer_id, started_at, ended_at, bytes_used, worker_bytes_reported, cost_usd, COALESCE(locked_price_per_gb, 1.50), active, billed
		 FROM sessions
		 WHERE id = $1 AND buyer_id = $2
		 FOR UPDATE`,
		sessionID, buyerID,
	).Scan(&session.ID, &session.BuyerID, &session.NodeID, &session.OfferID,
		&session.StartedAt, &session.EndedAt, &session.BytesUsed, &session.WorkerBytesReported, &session.CostUSD, &session.LockedPricePerGB, &session.Active, &session.Billed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrSessionNotFound
		}
		return nil, false, err
	}

	if additionalBytes < 0 {
		return nil, false, errors.New("additional bytes must be non-negative")
	}

	if session.Active && additionalBytes > 0 {
		additionalCost := float64(additionalBytes) / (1024 * 1024 * 1024) * session.LockedPricePerGB
		err = tx.QueryRow(
			`UPDATE sessions
			 SET bytes_used = bytes_used + $1, cost_usd = cost_usd + $2
			 WHERE id = $3
			 RETURNING bytes_used, cost_usd`,
			additionalBytes, additionalCost, sessionID,
		).Scan(&session.BytesUsed, &session.CostUSD)
		if err != nil {
			return nil, false, err
		}

		if _, err = tx.Exec(
			`INSERT INTO usage_logs (session_id, bytes) VALUES ($1, $2)`,
			sessionID, additionalBytes,
		); err != nil {
			return nil, false, err
		}
	}

	if session.Active {
		err = tx.QueryRow(
			`UPDATE sessions
			 SET active = false, ended_at = NOW()
			 WHERE id = $1
			 RETURNING ended_at, active`,
			sessionID,
		).Scan(&session.EndedAt, &session.Active)
		if err != nil {
			return nil, false, err
		}
	}

	// E3 cross-check: if the worker independently reported more bytes than the
	// gateway measured, use the worker's higher figure for billing. This prevents
	// an underreporting Gateway from leaving the worker underpaid. Log a warning
	// whenever the two counts diverge by more than 10% so ops can investigate.
	if session.WorkerBytesReported > session.BytesUsed {
		log.Printf("[E3] session=%s gateway_bytes=%d worker_bytes=%d — billing worker count (higher)",
			sessionID, session.BytesUsed, session.WorkerBytesReported)
		overageBytes := session.WorkerBytesReported - session.BytesUsed
		overageCost := float64(overageBytes) / (1024 * 1024 * 1024) * session.LockedPricePerGB
		if err = tx.QueryRow(
			`UPDATE sessions
			 SET bytes_used = worker_bytes_reported,
			     cost_usd   = cost_usd + $1
			 WHERE id = $2
			 RETURNING bytes_used, cost_usd`,
			overageCost, sessionID,
		).Scan(&session.BytesUsed, &session.CostUSD); err != nil {
			return nil, false, err
		}
	} else if session.BytesUsed > 0 && session.WorkerBytesReported > 0 {
		ratio := float64(session.WorkerBytesReported) / float64(session.BytesUsed)
		if ratio < 0.9 || ratio > 1.1 {
			log.Printf("[E3] session=%s byte count mismatch: gateway=%d worker=%d ratio=%.2f",
				sessionID, session.BytesUsed, session.WorkerBytesReported, ratio)
		}
	}

	charged := false
	if !session.Billed && session.CostUSD > 0 {
		res, err := tx.Exec(
			`UPDATE buyers
			 SET balance_usd = balance_usd - $1
			 WHERE id = $2 AND balance_usd >= $1`,
			session.CostUSD, session.BuyerID,
		)
		if err != nil {
			return nil, false, err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return nil, false, err
		}
		if rows == 0 {
			return nil, false, ErrInsufficientBuyerBalance
		}

		// 3-stream split distribution (Worker/Referrer/Treasury)
		var deviceID string
		if err := tx.QueryRow(`SELECT device_id FROM nodes WHERE id = $1`, session.NodeID).Scan(&deviceID); err != nil {
			return nil, false, err
		}
		if _, err := DistributeReward(tx, deviceID, session.CostUSD, "proxy_session", ""); err != nil {
			return nil, false, err
		}

		charged = true
	}
	if session.OfferID != nil && session.CostUSD > 0 {
		if err := SettleOffer(*session.OfferID, session.CostUSD); err != nil {
			return nil, false, err
		}
	}

	if !session.Billed {
		if _, err = tx.Exec(
			`UPDATE sessions SET billed = true WHERE id = $1`,
			sessionID,
		); err != nil {
			return nil, false, err
		}
		session.Billed = true
	}

	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return session, charged, nil
}

func GetBuyerSessions(buyerID string, limit int) ([]Session, error) {
	rows, err := db.DB.Query(
		`SELECT id, buyer_id, node_id, offer_id, started_at, ended_at, bytes_used, worker_bytes_reported, cost_usd, COALESCE(locked_price_per_gb, 1.50), active, billed
		 FROM sessions WHERE buyer_id = $1
		 ORDER BY started_at DESC LIMIT $2`,
		buyerID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.BuyerID, &s.NodeID, &s.OfferID,
			&s.StartedAt, &s.EndedAt, &s.BytesUsed, &s.WorkerBytesReported, &s.CostUSD, &s.LockedPricePerGB, &s.Active, &s.Billed); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}
