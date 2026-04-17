package main

import (
	"fmt"
	"log"
	"os"

	"exra/config"
	"exra/db"
	_ "github.com/lib/pq"
)

func main() {
	err := os.Chdir("../..")
	if err != nil {
		log.Fatal(err)
	}

	cfg := config.LoadConfig()
	db.Init(cfg.SupabaseURL)

	rows, err := db.DB.Query("SELECT column_name FROM information_schema.columns WHERE table_name = 'oracle_mint_queue'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Columns in oracle_mint_queue:")
	for rows.Next() {
		var col string
		rows.Scan(&col)
		fmt.Println("- " + col)
	}
}
