package main

import (
	"log"
	"os"

	"exra/config"
	"exra/db"
)

func main() {
	err := os.Chdir("../..")
	if err != nil {
		log.Fatal(err)
	}

	cfg := config.LoadConfig()
	db.Init(cfg.SupabaseURL)

	log.Println("Cleaning task_assignments...")
	_, err = db.DB.Exec("DELETE FROM task_assignments")
	if err != nil {
		log.Printf("Assignments cleanup error: %v", err)
	}

	log.Println("Resetting compute_tasks...")
	_, err = db.DB.Exec("UPDATE compute_tasks SET status = 'failed' WHERE status = 'pending' OR status = 'assigned'")
	if err != nil {
		log.Printf("Tasks reset error: %v", err)
	}

	log.Println("Cleanup complete.")
}
