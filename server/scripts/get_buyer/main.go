package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load("../../.env")
	dbURL := os.Getenv("SUPABASE_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var apiKey string
	err = db.QueryRow("SELECT api_key FROM buyers LIMIT 1").Scan(&apiKey)
	if err != nil {
		log.Fatal("No buyers found. Run a registration first or manually insert.")
	}

	fmt.Println(apiKey)
}
