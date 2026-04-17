package main

import (
	"flag"
	"log"
)

func main() {
	hubURL := flag.String("hub", "", "Hub WebSocket URL (e.g. ws://localhost:8080/ws)")
	flag.Parse()

	log.Println("--- Exra Desktop Client ---")

	// 1. Load config
	cfg := LoadConfig()
	if *hubURL != "" {
		cfg.HubURL = *hubURL
	}

	// 2. Hardware detection
	log.Println("[Main] Detecting hardware...")
	hw := GetHardwareInfo()
	log.Printf("[Main] Hardware: %s", hw.Summary())

	// 3. Start background worker
	log.Printf("[Main] Starting worker for Device ID: %s", cfg.DeviceID)
	log.Printf("[Main] Target Hub: %s", cfg.HubURL)

	StartWorker(cfg, hw)
}
