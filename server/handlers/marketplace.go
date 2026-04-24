package handlers

// marketplace.go — Public buyer-facing marketplace listing API.
//
// Auth model:
//   GET /api/marketplace/lots          — public, no auth (buyers browse).
//   GET /api/marketplace/lots/{id}     — public, single-lot detail.
//
// Buyers do NOT share an auth surface with workers (TMA/Telegram).
// Purchase is initiated via the existing POST /api/offers (BuyerAuth API key).
// This handler is read-only — no balance or state mutations happen here.

import (
	"encoding/base64"
	"encoding/json"
	"exra/db"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// lotCursor holds the sort-key values of the last row on the previous page.
// Sort order: (online_rank ASC, gear_score DESC, price_per_gb ASC, id ASC).
// Encoding is base64(JSON) so the client treats it as an opaque token.
type lotCursor struct {
	OnlineRank int     `json:"o"` // 0=online, 1=other
	GearScore  float64 `json:"g"`
	PricePerGB float64 `json:"p"`
	ID         string  `json:"id"`
}

func encodeCursor(c lotCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (lotCursor, bool) {
	if s == "" {
		return lotCursor{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return lotCursor{}, false
	}
	var c lotCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return lotCursor{}, false
	}
	return c, true
}

type MarketplaceLot struct {
	ID            string  `json:"id"`
	DeviceID      string  `json:"device_id"`
	PricePerGB    float64 `json:"price_per_gb"`
	BandwidthMbps int     `json:"bandwidth_mbps"`
	GearScore     float64 `json:"gear_score"`
	IdentityTier  string  `json:"identity_tier"`
	PopSessions   int     `json:"pop_sessions"`
	Country       string  `json:"country"`
	DeviceType    string  `json:"device_type"`
	IsResidential bool    `json:"is_residential"`
	NodeStatus    string  `json:"node_status"`
}

const marketplaceLotColumns = `
	wl.id, wl.device_id, wl.price_per_gb, wl.bandwidth_mbps,
	wl.gear_score, wl.identity_tier, wl.pop_sessions,
	COALESCE(n.country,'')           AS country,
	COALESCE(n.device_type,'')       AS device_type,
	COALESCE(n.is_residential,false) AS is_residential,
	COALESCE(n.status,'offline')     AS node_status`

func scanMarketplaceLot(rows interface {
	Scan(dest ...any) error
}) (MarketplaceLot, error) {
	var lot MarketplaceLot
	err := rows.Scan(
		&lot.ID, &lot.DeviceID, &lot.PricePerGB, &lot.BandwidthMbps,
		&lot.GearScore, &lot.IdentityTier, &lot.PopSessions,
		&lot.Country, &lot.DeviceType, &lot.IsResidential, &lot.NodeStatus,
	)
	return lot, err
}

// GET /api/marketplace/lots — public listing for buyers.
//
// Query params (all optional):
//   max_price   float   — upper price bound ($/GB)
//   min_tier    string  — anon | basic | peak
//   country     string  — ISO-3166-1 alpha-2 (e.g. US, DE)
//   residential bool    — true = residential IPs only
//   limit       int     — 1–100, default 50
//   cursor      string  — opaque next-page token from previous response
//
// Pagination is cursor-based (keyset) over (online_rank, gear_score DESC,
// price_per_gb ASC, id ASC) — stable under concurrent inserts/deletes.
// Response: {"lots":[...], "count": N, "next_cursor": "..." | null}
// Online nodes appear first; offline (but not frozen) nodes follow.
func MarketplaceListLots(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	minTier := q.Get("min_tier")
	country := q.Get("country")
	maxPrice, _ := strconv.ParseFloat(q.Get("max_price"), 64)
	residentialOnly := q.Get("residential") == "true"

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	if minTier != "" && minTier != "anon" && minTier != "basic" && minTier != "peak" {
		jsonError(w, "min_tier must be one of: anon, basic, peak", http.StatusBadRequest)
		return
	}

	cur, hasCursor := decodeCursor(q.Get("cursor"))

	var rows interface {
		Next() bool
		Scan(...any) error
		Close() error
	}
	var err error

	if hasCursor {
		// Keyset condition: (online_rank, -gear_score, price_per_gb, id) > cursor values.
		// Using negated gear_score so a single ROW() comparison handles mixed ASC/DESC.
		rows, err = db.DB.Query(`
			SELECT`+marketplaceLotColumns+`
			FROM worker_listings wl
			INNER JOIN nodes n ON n.device_id = wl.device_id
			WHERE wl.status = 'active'
			  AND n.status   != 'frozen'
			  AND ($1 = ''   OR wl.identity_tier = $1)
			  AND ($2 = 0    OR wl.price_per_gb <= $2)
			  AND ($3 = ''   OR n.country = $3)
			  AND (NOT $4    OR n.is_residential = true)
			  AND (
			    CASE WHEN n.status='online' THEN 0 ELSE 1 END,
			    -wl.gear_score,
			    wl.price_per_gb,
			    wl.id
			  ) > ($6::int, $7::float8, $8::float8, $9::text)
			ORDER BY
			    CASE WHEN n.status = 'online' THEN 0 ELSE 1 END,
			    wl.gear_score DESC,
			    wl.price_per_gb ASC,
			    wl.id ASC
			LIMIT $5`,
			minTier, maxPrice, country, residentialOnly, limit,
			cur.OnlineRank, -cur.GearScore, cur.PricePerGB, cur.ID,
		)
	} else {
		rows, err = db.DB.Query(`
			SELECT`+marketplaceLotColumns+`
			FROM worker_listings wl
			INNER JOIN nodes n ON n.device_id = wl.device_id
			WHERE wl.status = 'active'
			  AND n.status   != 'frozen'
			  AND ($1 = ''   OR wl.identity_tier = $1)
			  AND ($2 = 0    OR wl.price_per_gb <= $2)
			  AND ($3 = ''   OR n.country = $3)
			  AND (NOT $4    OR n.is_residential = true)
			ORDER BY
			    CASE WHEN n.status = 'online' THEN 0 ELSE 1 END,
			    wl.gear_score DESC,
			    wl.price_per_gb ASC,
			    wl.id ASC
			LIMIT $5`,
			minTier, maxPrice, country, residentialOnly, limit,
		)
	}
	if err != nil {
		log.Printf("marketplace-lots: query err: %v", err)
		jsonError(w, "failed to load marketplace", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	lots := make([]MarketplaceLot, 0, limit)
	for rows.Next() {
		lot, err := scanMarketplaceLot(rows)
		if err != nil {
			log.Printf("marketplace-lots: scan err: %v", err)
			continue
		}
		lots = append(lots, lot)
	}

	var nextCursor *string
	if len(lots) == limit {
		last := lots[len(lots)-1]
		onlineRank := 1
		if last.NodeStatus == "online" {
			onlineRank = 0
		}
		tok := encodeCursor(lotCursor{
			OnlineRank: onlineRank,
			GearScore:  last.GearScore,
			PricePerGB: last.PricePerGB,
			ID:         last.ID,
		})
		nextCursor = &tok
	}

	jsonResponse(w, map[string]any{
		"lots":        lots,
		"count":       len(lots),
		"next_cursor": nextCursor,
	}, http.StatusOK)
}

// GET /api/marketplace/lots/{id} — single lot detail (public).
func MarketplaceGetLot(w http.ResponseWriter, r *http.Request) {
	lotID := mux.Vars(r)["id"]
	if lotID == "" {
		jsonError(w, "lot id required", http.StatusBadRequest)
		return
	}

	row := db.DB.QueryRow(`
		SELECT`+marketplaceLotColumns+`
		FROM worker_listings wl
		INNER JOIN nodes n ON n.device_id = wl.device_id
		WHERE wl.id = $1 AND wl.status != 'deleted'`, lotID)

	lot, err := scanMarketplaceLot(row)
	if err != nil {
		jsonError(w, "listing not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, lot, http.StatusOK)
}
