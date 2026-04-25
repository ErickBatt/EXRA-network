package handlers

import (
	"encoding/json"
	"exra/gwclaims"
	"exra/hub"
	"exra/middleware"
	"exra/models"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"os"
	"sort"
	"time"
)

type MatcherHandler struct {
	Hub *hub.Hub
}

// scoredCandidate is a ranked match candidate: the parsed node plus its raw
// JSON (needed verbatim as the Redis ZSET member for atomic ZREM).
type scoredCandidate struct {
	node    models.PublicNode
	rawJSON string
	score   float64
}

func (h *MatcherHandler) CreateOfferAndMatch(w http.ResponseWriter, r *http.Request) {
	var offer struct {
		ID       string  `json:"id"`
		Price    float64 `json:"price"`
		Country  string  `json:"country"`
		TargetGB float64 `json:"target_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		jsonError(w, "invalid offer body", http.StatusBadRequest)
		return
	}

	// 0. Circuit Breaker Check
	if h.Hub != nil && h.Hub.IsGlobalPause() {
		jsonError(w, "circuit_breaker_active: Network is currently under maintenance", http.StatusServiceUnavailable)
		return
	}

	// 1. Get real buyer from context (Middleware: BuyerAuth)
	buyer := middleware.BuyerFromContext(r)
	if buyer == nil {
		jsonError(w, "unauthorized: buyer not found in context", http.StatusUnauthorized)
		return
	}

	// 2. Get avg price for country
	avgPrice, err := models.GetMarketAvgPrice(offer.Country)
	if err != nil || avgPrice <= 0 {
		avgPrice = 1.50 // fallback
	}

	// 3. Fetch top nodes from Redis ZSET (Tier A, then B if empty)
	rsTier := "A"
	nodesRaw, err := h.Hub.GetDiscoveryNodes(r.Context(), offer.Country, rsTier, 10)
	if err != nil || len(nodesRaw) == 0 {
		rsTier = "B"
		nodesRaw, _ = h.Hub.GetDiscoveryNodes(r.Context(), offer.Country, rsTier, 10)
	}
	if len(nodesRaw) == 0 {
		jsonError(w, "no nodes available for matching in this region", http.StatusNotFound)
		return
	}

	// 4. Generate session ID before scoring so the HRW tiebreaker can use it
	// as a per-session seed (deterministic spread across equal-quality nodes).
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())

	// Score all candidates, sort desc. We need the full ranking (not just
	// the argmax) so that if the top pick loses the atomic claim race we can
	// fall back to the next best candidate in a single matcher pass. This is
	// part of the AUDIT §1 B1 fix.
	candidates := make([]scoredCandidate, 0, len(nodesRaw))
	for _, nodeJSON := range nodesRaw {
		var node models.PublicNode
		if err := json.Unmarshal([]byte(nodeJSON), &node); err != nil {
			continue
		}
		candidates = append(candidates, scoredCandidate{
			node:    node,
			rawJSON: nodeJSON,
			score:   calculateBidScore(sessionID, offer.Price, avgPrice, node),
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	// 5. Atomic claim: remove the chosen node from the pool and write a TTL
	// lease BEFORE issuing any JWT (AUDIT §1 B1). If the top pick is already
	// leased by a concurrent matcher, fall through to the next best.
	const claimTTL = 60 * time.Second
	var bestNode models.PublicNode
	var bestScore float64
	claimed := false
	for _, c := range candidates {
		ok, claimErr := h.Hub.AtomicClaimNode(r.Context(), offer.Country, rsTier, c.node.DeviceID, c.rawJSON, sessionID, claimTTL)
		if claimErr != nil {
			continue
		}
		if ok {
			bestNode = c.node
			bestScore = c.score
			claimed = true
			break
		}
	}
	if !claimed {
		jsonError(w, "no nodes available: all candidates are currently leased", http.StatusServiceUnavailable)
		return
	}

	// 6. Balance hold — atomic SELECT/UPDATE ... WHERE balance_usd >= cost.
	// Must happen AFTER claim and BEFORE JWT issuance (AUDIT §1 B3). If the
	// buyer cannot cover the hold, release the node lease and return 402.
	cost := offer.Price * offer.TargetGB
	if err := models.HoldBalance(buyer.ID, cost); err != nil {
		_ = h.Hub.ReleaseNodeLease(r.Context(), bestNode.DeviceID)
		if err == models.ErrInsufficientBalance {
			jsonError(w, "insufficient buyer balance for requested session", http.StatusPaymentRequired)
		} else {
			jsonError(w, "balance hold failed", http.StatusInternalServerError)
		}
		return
	}

	// 7. Generate Gateway JWTs (EdDSA, no hardcoded fallback — AUDIT §1 D1).
	signedBuyerToken, err := gwclaims.Sign(sessionID, buyer.ID, "buyer", gwclaims.DefaultTTL)
	if err != nil {
		_ = h.Hub.ReleaseNodeLease(r.Context(), bestNode.DeviceID)
		_ = models.ReleaseBalanceHold(buyer.ID, cost)
		jsonError(w, "token signing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	signedNodeToken, err := gwclaims.Sign(sessionID, "", "node", gwclaims.DefaultTTL)
	if err != nil {
		_ = h.Hub.ReleaseNodeLease(r.Context(), bestNode.DeviceID)
		_ = models.ReleaseBalanceHold(buyer.ID, cost)
		jsonError(w, "token signing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 8. Persist session state in Redis. price_per_gb is read by the Gateway
	// billing settlement path (AUDIT §1 G3) so both planes agree on the tariff.
	_ = h.Hub.CreateSessionInRedis(r.Context(), sessionID, map[string]interface{}{
		"buyer_id":     buyer.ID,
		"node_did":     bestNode.ID,
		"credits":      cost,
		"price_per_gb": offer.Price,
		"status":       "starting",
	})

	// 9. Notify the node. Non-blocking: if its send queue is full, release the
	// lease + hold rather than pinning the HTTP goroutine (AUDIT §2 B4).
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "wss://gateway.exra.network/gateway"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":        "gateway_connect",
		"session_id":  sessionID,
		"gateway_url": gatewayURL + "?jwt=" + signedNodeToken,
	})
	if client, ok := h.Hub.GetClient(bestNode.DeviceID); ok {
		select {
		case client.Send <- payload:
		default:
			_ = h.Hub.ReleaseNodeLease(r.Context(), bestNode.DeviceID)
			_ = models.ReleaseBalanceHold(buyer.ID, cost)
			jsonError(w, "selected node is backpressured", http.StatusServiceUnavailable)
			return
		}
	}

	jsonResponse(w, map[string]any{
		"session_id":  sessionID,
		"gateway_url": gatewayURL + "?jwt=" + signedBuyerToken,
		"node":        bestNode,
		"score":       bestScore,
	}, http.StatusOK)
}

// calculateBidScore implements the matchmaking formula from
// MASTER_PLAN/v2.4.1: 50% Reputation Score + 50% Latency/Geo. Because
// per-node RTT telemetry is not yet collected, Latency/Geo is currently
// approximated as 0.25*Uptime + 0.25*priceFitness where priceFitness is a
// clamped ratio of offer.Price to the country's average.
//
// A small HRW (Highest Random Weight) tiebreaker (≤5% of total score) is
// added using fnv32a(sessionID+nodeDeviceID). This spreads load across
// equivalently-scored nodes per session without overriding quality selection.
func calculateBidScore(sessionID string, offerPrice, avgPrice float64, node models.PublicNode) float64 {
	rsScore := node.RSScore / 1000.0
	if rsScore < 0 {
		rsScore = 0
	}
	if rsScore > 1 {
		rsScore = 1
	}

	uptimeScore := node.Uptime
	if uptimeScore < 0 {
		uptimeScore = 0
	}
	if uptimeScore > 1 {
		uptimeScore = 1
	}

	priceFitness := 0.0
	if avgPrice > 0 {
		priceFitness = offerPrice / avgPrice
	}
	if priceFitness < 0 {
		priceFitness = 0
	}
	if priceFitness > 1 {
		priceFitness = 1
	}

	baseScore := 0.5*rsScore + 0.25*uptimeScore + 0.25*priceFitness

	// HRW tiebreaker: deterministic per-(session, node) hash in [0, 0.05].
	h := fnv.New32a()
	h.Write([]byte(sessionID + node.DeviceID))
	hrw := float64(h.Sum32()) / math.MaxUint32 * 0.05

	return baseScore + hrw
}
