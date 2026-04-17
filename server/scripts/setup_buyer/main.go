package main

import (
	"fmt"
	"log"
	"os"

	"exra/config"
	"exra/db"
	"exra/models"
)

func main() {
	// Change working directory to server root so config loading works
	err := os.Chdir("../..")
	if err != nil {
		log.Fatalf("failed to change dir: %v", err)
	}

	cfg := config.LoadConfig()
	db.Init(cfg.SupabaseURL)

	email := "test@exra.net"
	apiKey := "test-buyer-serious-guys"
	
	// Check if exists
	buyer, err := models.GetBuyerByAPIKey(apiKey)
	if err == nil {
		fmt.Printf("Test buyer already exists: %s\n", buyer.APIKey)
		return
	}

	// Create or update balance
	_, err = db.DB.Exec(`
		INSERT INTO buyers (api_key, email, balance_usd) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (email) DO UPDATE SET balance_usd = 100`, 
		apiKey, email, 100.0)
	
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Test buyer setup complete with API key: " + apiKey)
}
