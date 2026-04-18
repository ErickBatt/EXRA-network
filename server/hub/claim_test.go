package hub

// claim_test.go — integration-level proof for the B1 fix.
//
// Covers AUDIT_MARKETPLACE_v2.4.1 §1 B1: the Matcher previously picked the
// best node from a ZSET snapshot and handed out a Gateway JWT without any
// step that actually removed the chosen node from the pool. Two concurrent
// matcher calls therefore picked the same node. The fix is Hub.AtomicClaimNode,
// a Redis Lua script that in one transaction checks the lease is free,
// ZREMs the member from the discovery ZSET, and SETs the lease with TTL.
//
// This test spins up N concurrent claim attempts on the same node and
// asserts that exactly one of them wins. Requires a real Redis on
// localhost:6379 — skipped otherwise (same pattern as hub_redis_test.go).

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestHub_AtomicClaimNode_IsAtomic(t *testing.T) {
	rc := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx := context.Background()
	if err := rc.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available on localhost:6379, skipping B1 integration test")
	}
	defer rc.Close()

	h := &Hub{rc: rc}

	const (
		country    = "TH"
		tier       = "A"
		deviceID   = "node-under-contention"
		concurrent = 50
	)
	zsetKey := fmt.Sprintf(KeyPrefixNodes, country, tier)
	leaseKey := "lease:node:" + deviceID
	nodeJSON := fmt.Sprintf(`{"device_id":"%s","price_per_gb":1.5}`, deviceID)

	// Clean slate + seed the discovery ZSET with the contested node.
	_ = rc.Del(ctx, zsetKey, leaseKey).Err()
	if err := h.SyncNodeToRedis(ctx, country, tier, 1.5, nodeJSON); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	t.Cleanup(func() {
		_ = rc.Del(context.Background(), zsetKey, leaseKey).Err()
	})

	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ok, err := h.AtomicClaimNode(ctx, country, tier, deviceID, nodeJSON,
				fmt.Sprintf("sess_%d", idx), 30*time.Second)
			if err != nil {
				t.Errorf("goroutine %d: claim error: %v", idx, err)
				return
			}
			if ok {
				winners.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if w := winners.Load(); w != 1 {
		t.Fatalf("BUG B1 regression: expected exactly 1 winner among %d concurrent claims, got %d", concurrent, w)
	}

	// Confirm the ZSET no longer contains the member.
	remaining, err := h.GetDiscoveryNodes(ctx, country, tier, 10)
	if err != nil {
		t.Fatalf("GetDiscoveryNodes: %v", err)
	}
	for _, m := range remaining {
		if m == nodeJSON {
			t.Fatalf("BUG B1 regression: node still present in ZSET after successful claim")
		}
	}

	// Confirm the lease key is set.
	if v, err := rc.Get(ctx, leaseKey).Result(); err != nil || v == "" {
		t.Fatalf("BUG B1 regression: lease key missing after successful claim (val=%q err=%v)", v, err)
	}
}
