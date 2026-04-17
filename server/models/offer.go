package models

import (
	"database/sql"
	"errors"
	"exra/db"
	"time"
)

type Offer struct {
	ID            string    `json:"id"`
	BuyerID       string    `json:"buyer_id"`
	Country       string    `json:"country"`
	TargetGB      float64   `json:"target_gb"`
	MaxPricePerGB float64   `json:"max_price_per_gb"`
	Status        string    `json:"status"`
	ReservedEXRA  float64   `json:"reserved_exra"`
	SettledEXRA   float64   `json:"settled_exra"`
	CreatedAt     time.Time `json:"created_at"`
}

var ErrOfferNotFound = errors.New("offer not found")

func CreateOffer(buyerID, country string, targetGB, maxPricePerGB float64) (*Offer, error) {
	o := &Offer{}
	reserved := targetGB * maxPricePerGB
	err := db.DB.QueryRow(
		`INSERT INTO offers (buyer_id, country, target_gb, max_price_per_gb, status, reserved_exra)
		 VALUES ($1, $2, $3, $4, 'pending', $5)
		 RETURNING id, buyer_id, country, target_gb, max_price_per_gb, status, reserved_exra, settled_exra, created_at`,
		buyerID, country, targetGB, maxPricePerGB, reserved,
	).Scan(&o.ID, &o.BuyerID, &o.Country, &o.TargetGB, &o.MaxPricePerGB, &o.Status, &o.ReservedEXRA, &o.SettledEXRA, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func ListOffersByBuyer(buyerID string, limit int) ([]Offer, error) {
	rows, err := db.DB.Query(
		`SELECT id, buyer_id, country, target_gb, max_price_per_gb, status, reserved_exra, settled_exra, created_at
		 FROM offers
		 WHERE buyer_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		buyerID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Offer, 0)
	for rows.Next() {
		var o Offer
		if err := rows.Scan(&o.ID, &o.BuyerID, &o.Country, &o.TargetGB, &o.MaxPricePerGB, &o.Status, &o.ReservedEXRA, &o.SettledEXRA, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func AssignOffer(offerID string) (*Offer, *Node, *Session, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, nil, nil, err
	}
	defer tx.Rollback()

	var o Offer
	err = tx.QueryRow(
		`SELECT id, buyer_id, country, target_gb, max_price_per_gb, status, reserved_exra, settled_exra, created_at
		 FROM offers WHERE id = $1 FOR UPDATE`,
		offerID,
	).Scan(&o.ID, &o.BuyerID, &o.Country, &o.TargetGB, &o.MaxPricePerGB, &o.Status, &o.ReservedEXRA, &o.SettledEXRA, &o.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, ErrOfferNotFound
		}
		return nil, nil, nil, err
	}
	if o.Status != "pending" {
		return nil, nil, nil, errors.New("offer is not pending")
	}

	node, err := getBestNodeForOfferTx(tx, o.Country, o.MaxPricePerGB)
	if err != nil {
		return nil, nil, nil, err
	}

	var session Session
	lockedPrice := node.PricePerGB
	if node.AutoPrice {
		if avg, err := GetMarketAvgPrice(node.Country); err == nil {
			lockedPrice = avg
		}
	}
	if err := tx.QueryRow(
		`INSERT INTO sessions (buyer_id, node_id, offer_id, locked_price_per_gb)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, buyer_id, node_id, offer_id, started_at, bytes_used, cost_usd, locked_price_per_gb, active, billed`,
		o.BuyerID, node.ID, o.ID, lockedPrice,
	).Scan(&session.ID, &session.BuyerID, &session.NodeID, &session.OfferID, &session.StartedAt, &session.BytesUsed, &session.CostUSD, &session.LockedPricePerGB, &session.Active, &session.Billed); err != nil {
		return nil, nil, nil, err
	}
	if _, err := tx.Exec(`UPDATE offers SET status='assigned' WHERE id=$1`, o.ID); err != nil {
		return nil, nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, nil, err
	}
	o.Status = "assigned"
	return &o, node, &session, nil
}

func SettleOffer(offerID string, settledEXRA float64) error {
	res, err := db.DB.Exec(
		`UPDATE offers
		 SET settled_exra = settled_exra + $1,
		     status = CASE WHEN settled_exra + $1 >= reserved_exra THEN 'settled' ELSE status END
		 WHERE id = $2`,
		settledEXRA, offerID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrOfferNotFound
	}
	return nil
}

func getBestNodeForOfferTx(tx *sql.Tx, country string, maxPrice float64) (*Node, error) {
	node := &Node{}
	err := tx.QueryRow(
		`SELECT n.id, COALESCE(n.device_id, ''), COALESCE(n.ip, ''), n.address, n.port, n.country,
		        COALESCE(n.device_type, ''), COALESCE(n.device_tier,'network'), COALESCE(n.is_residential, true),
		        COALESCE(n.asn_org, ''), COALESCE(n.status, 'online'), COALESCE(n.traffic_bytes, 0), n.bandwidth_mbps,
		        COALESCE(n.cpu_model,''), COALESCE(n.cpu_cores,0), COALESCE(n.vram_mb,0), COALESCE(n.ram_mb,0),
		        n.active, COALESCE(n.price_per_gb, 1.50), COALESCE(n.auto_price, true), COALESCE(n.last_seen, NOW()), n.last_heartbeat, n.created_at
		 FROM nodes n
		 WHERE n.active = true
		   AND n.status = 'online'
		   AND n.last_heartbeat > NOW() - INTERVAL '2 minutes'
		   AND ($1 = '' OR n.country = $1)
		   AND (COALESCE(n.price_per_gb, 1.50) <= $2 OR COALESCE(n.auto_price, true) = true)
		 ORDER BY COALESCE(n.price_per_gb, 1.50) ASC, n.bandwidth_mbps DESC, n.last_heartbeat DESC
		 LIMIT 1`,
		country, maxPrice,
	).Scan(&node.ID, &node.DeviceID, &node.IP, &node.Address, &node.Port, &node.Country,
		&node.DeviceType, &node.DeviceTier, &node.IsResidential, &node.ASNOrg, &node.Status, &node.TrafficBytes, &node.BandwidthMbps,
		&node.CPUModel, &node.CPUCores, &node.VRAMMB, &node.RAMMB,
		&node.Active, &node.PricePerGB, &node.AutoPrice, &node.LastSeen, &node.LastHeartbeat, &node.CreatedAt)
	if err != nil {
		return nil, err
	}
	return node, nil
}
