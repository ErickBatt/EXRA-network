package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Billing glue for the v2.1 data plane (AUDIT_MARKETPLACE_v2.4.1 §1 G3).
//
// The old `handlers/proxy.go` HTTP path decremented credits inline; the new
// Gateway path (this binary) used to forward bytes through Bridge without
// ever touching the session credit counter, so buyers received unlimited
// traffic for a single up-front allowance. Here we open a Redis connection
// at startup and expose a small helper that converts settled byte counts to
// a `sessions:<sid>` credit decrement.
//
// Price is taken from the session hash written by the Control Plane matcher:
// fields `credits` (initial allowance, USD) and `price_per_gb` if present,
// otherwise the env fallback GATEWAY_PRICE_PER_GB.

var (
	redisClient    *redis.Client
	fallbackPrice  float64 = 1.50
)

func initBilling() {
	if v := os.Getenv("GATEWAY_PRICE_PER_GB"); v != "" {
		if p, err := strconv.ParseFloat(v, 64); err == nil && p > 0 {
			fallbackPrice = p
		}
	}

	url := os.Getenv("REDIS_URL")
	if url == "" {
		log.Println("[Gateway] REDIS_URL not set; billing settlement disabled (dev mode)")
		return
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("[Gateway] REDIS_URL parse failed: %v; billing settlement disabled", err)
		return
	}
	rc := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rc.Ping(pingCtx).Err(); err != nil {
		log.Printf("[Gateway] Redis ping failed: %v; billing settlement disabled", err)
		return
	}
	redisClient = rc
	log.Println("[Gateway] Billing settlement enabled via Redis")

	// Wire sessionKnownFn to Redis HEXISTS so Stitch fast-fails unknown or
	// already-settled sessions instead of creating orphan waiters that hang
	// for firstPartyTimeout (AUDIT §1 A1 alternative path).
	sessionKnownFn = func(sessionID string) bool {
		if sessionID == "" {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		ok, err := redisClient.HExists(ctx, "sessions:"+sessionID, "buyer_id").Result()
		if err != nil {
			// Fail-open: a transient Redis blip shouldn't refuse legit traffic.
			log.Printf("[Gateway] sessionKnownFn HEXISTS(%s): %v (fail-open)", sessionID, err)
			return true
		}
		return ok
	}
}

// settleSession deducts bytes-worth of credits from the session hash. Called
// once per Bridge close. The deduction is best-effort: if Redis is down we
// log and move on — losing the settlement is preferable to keeping the
// Gateway goroutine alive.
func settleSession(sessionID string, totalBytes int64) {
	if redisClient == nil || sessionID == "" || totalBytes <= 0 {
		return
	}
	key := "sessions:" + sessionID

	price := fallbackPrice
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if v, err := redisClient.HGet(ctx, key, "price_per_gb").Float64(); err == nil && v > 0 {
		price = v
	}

	cost := float64(totalBytes) / (1024 * 1024 * 1024) * price
	if cost <= 0 {
		return
	}
	remaining, err := redisClient.HIncrByFloat(ctx, key, "credits", -cost).Result()
	if err != nil {
		log.Printf("[Gateway] settle HIncrByFloat(%s): %v", key, err)
		return
	}
	_ = redisClient.HSet(ctx, key, "bytes_used", totalBytes, "last_settled_at", time.Now().Unix()).Err()
	log.Printf("[Gateway] Settled session=%s bytes=%d cost=%.6f remaining=%.6f", sessionID, totalBytes, cost, remaining)
}
