package hub

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestHubRedisOperations(t *testing.T) {
	// We need a test Redis instance. If not available, we skip.
	// In some environments, we use miniredis. Here we assume local redis for testing.
	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()
	if err := rc.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available on localhost:6379, skipping test")
	}
	defer rc.Close()

	h := &Hub{rc: rc}

	t.Run("NodeSyncAndDiscovery", func(t *testing.T) {
		country := "TH"
		tier := "A"
		nodeJSON := `{"id": "node-1", "price": 0.3}`
		
		err := h.SyncNodeToRedis(ctx, country, tier, 0.3, nodeJSON)
		if err != nil {
			t.Fatalf("SyncNodeToRedis failed: %v", err)
		}

		nodes, err := h.GetDiscoveryNodes(ctx, country, tier, 10)
		if err != nil {
			t.Fatalf("GetDiscoveryNodes failed: %v", err)
		}

		found := false
		for _, n := range nodes {
			if n == nodeJSON {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Node not found in discovery list")
		}

		// Cleanup
		h.RemoveNodeFromRedis(ctx, country, tier, nodeJSON)
	})

	t.Run("SessionBilling", func(t *testing.T) {
		jwt := "test-jwt-123"
		data := map[string]interface{}{
			"buyer_id": "buyer-1",
			"credits":  10.0,
		}

		err := h.CreateSessionInRedis(ctx, jwt, data)
		if err != nil {
			t.Fatalf("CreateSessionInRedis failed: %v", err)
		}

		newBalance, err := h.DeductSessionBalance(ctx, jwt, 1.5)
		if err != nil {
			t.Fatalf("DeductSessionBalance failed: %v", err)
		}
		if newBalance != 8.5 {
			t.Errorf("Expected balance 8.5, got %f", newBalance)
		}

		// Cleanup
		rc.Del(ctx, "sessions:"+jwt)
	})
}
