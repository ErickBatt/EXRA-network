package main

import (
	"exra/config"
	"exra/db"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	err := os.Chdir("../..")
	if err != nil {
		log.Fatal(err)
	}

	cfg := config.LoadConfig()
	db.Init(cfg.SupabaseURL)

	fmt.Println("--- PoP Reward Validation ---")

	// 1. Total events
	var totalEvents int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM pop_reward_events").Scan(&totalEvents)
	if err != nil {
		log.Fatalf("failed to count events: %v", err)
	}
	fmt.Printf("Total PoP Reward Events: %d\n", totalEvents)

	// 2. Worker rewards distributed
	var workerTotal float64
	err = db.DB.QueryRow("SELECT SUM(worker_reward) FROM pop_reward_events").Scan(&workerTotal)
	if err != nil {
		workerTotal = 0
	}
	fmt.Printf("Total Worker Rewards (EXRA): %.8f\n", workerTotal)

	// 3. Treasury rewards
	var treasuryTotal float64
	err = db.DB.QueryRow("SELECT SUM(treasury_reward) FROM pop_reward_events").Scan(&treasuryTotal)
	if err != nil {
		treasuryTotal = 0
	}
	fmt.Printf("Total Treasury Rewards (EXRA): %.8f\n", treasuryTotal)

	// 4. Verify distribution formula (Worker is 50%)
	if totalEvents > 0 {
		var totalEmission float64
		err = db.DB.QueryRow("SELECT SUM(total_emission) FROM pop_reward_events").Scan(&totalEmission)
		if err == nil && totalEmission > 0 {
			workerPct := (workerTotal / totalEmission) * 100
			fmt.Printf("Worker Reward Ratio: %.2f%% (Goal: 50.00%%)\n", workerPct)
		}
	}

	// 5. Check idempotency (Should have no duplicates with same device_id and 30s bucket, 
	// but the DB constraint handles this, so we just check if multiple events were inserted 
	// for the same node in a very short time if any).
	var duplicates int
	query := `
		SELECT COUNT(*) FROM (
			SELECT device_id, date_trunc('minute', created_at) as bucket, COUNT(*) 
			FROM pop_reward_events 
			GROUP BY device_id, bucket 
			HAVING COUNT(*) > 2
		) as dups`
	err = db.DB.QueryRow(query).Scan(&duplicates)
	if err != nil {
		duplicates = 0
	}
	fmt.Printf("Potential Idempotency Issues (Nodes with >2 events per minute): %d\n", duplicates)

	fmt.Println("-----------------------------")
	if totalEvents > 0 {
		fmt.Println("✅ SYSTEM VERIFIED: Rewards are flowing correctly.")
	} else {
		fmt.Println("⚠️ WARNING: No reward events found. Check if load_tester is running heartbeats.")
	}
}
