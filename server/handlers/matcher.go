package handlers

import (
	"encoding/json"
	"exra/hub"
	"exra/middleware"
	"exra/models"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var gatewaySecret = []byte(os.Getenv("GATEWAY_JWT_SECRET"))

func init() {
	if len(gatewaySecret) == 0 {
		gatewaySecret = []byte("default_gateway_secret_change_me_in_production")
	}
}

type GatewayClaims struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"` // "node" or "buyer"
	jwt.RegisteredClaims
}

type MatcherHandler struct {
	Hub *hub.Hub
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

	// 3. Fetch top nodes from Redis ZSET (Tier A)
	nodesRaw, err := h.Hub.GetDiscoveryNodes(r.Context(), offer.Country, "A", 10)
	if err != nil || len(nodesRaw) == 0 {
		// Try Tier B if A is empty
		nodesRaw, _ = h.Hub.GetDiscoveryNodes(r.Context(), offer.Country, "B", 10)
	}
	
	if len(nodesRaw) == 0 {
		jsonError(w, "no nodes available for matching in this region", http.StatusNotFound)
		return
	}

	// 4. Scoring
	var bestNode models.PublicNode
	bestScore := -1.0

	for _, nodeJSON := range nodesRaw {
		var node models.PublicNode
		if err := json.Unmarshal([]byte(nodeJSON), &node); err != nil {
			continue
		}

		score := calculateBidScore(offer.Price, avgPrice, node)
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	// 5. Generate Gateway JWTs
	sessionID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	
	// Token for Buyer
	buyerClaims := GatewayClaims{
		SessionID: sessionID,
		Role:      "buyer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}
	buyerToken := jwt.NewWithClaims(jwt.SigningMethodHS256, buyerClaims)
	signedBuyerToken, _ := buyerToken.SignedString(gatewaySecret)

	// Token for Node
	nodeClaims := GatewayClaims{
		SessionID: sessionID,
		Role:      "node",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	}
	nodeToken := jwt.NewWithClaims(jwt.SigningMethodHS256, nodeClaims)
	signedNodeToken, _ := nodeToken.SignedString(gatewaySecret)

	// 6. Update Redis session with ACTUAL Buyer ID
	h.Hub.CreateSessionInRedis(r.Context(), sessionID, map[string]interface{}{
		"buyer_id": buyer.ID,
		"node_did": bestNode.ID,
		"credits":  offer.Price * offer.TargetGB,
		"status":   "starting",
	})

	// 7. Notify Node via WS Hub to connect to Gateway (with Node JWT)
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "wss://gateway.exra.network/gateway"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":        "gateway_connect",
		"session_id":  sessionID,
		"gateway_url": gatewayURL + "?jwt=" + signedNodeToken,
	})
	if client, ok := h.Hub.GetClient(bestNode.ID); ok {
		client.Send <- payload
	}

	jsonResponse(w, map[string]any{
		"session_id":  sessionID,
		"gateway_url": gatewayURL + "?jwt=" + signedBuyerToken,
		"node":        bestNode,
		"score":       bestScore,
	}, http.StatusOK)
}

func calculateBidScore(offerPrice, avgPrice float64, node models.PublicNode) float64 {
	// Formula: 0.4*(offer.Price/avgPrice) + 0.3*(node.RS/1000) + 0.2*node.Uptime + 0.1*peakBonus + rand.Float64()*0.05
	
	peakBonus := 0.0
	if node.RSTier == "A" {
		peakBonus = 0.1
	}

	score := 0.4*(offerPrice/avgPrice) + 0.3*(node.RSScore/1000.0) + 0.2*node.Uptime + 0.1*peakBonus + rand.Float64()*0.05
	return score
}
