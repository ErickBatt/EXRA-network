// cmd/create_admin — bootstrap tool to create the first admin user.
//
// Usage:
//   go run ./cmd/create_admin -email admin@example.com -role admin_root
//
// Roles: admin_root, admin_finance, admin_ops, admin_readonly
//
// The generated API key is printed once. Store it securely — it cannot be recovered.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"exra/config"
	"exra/db"
)

var validRoles = map[string]bool{
	"admin_root":     true,
	"admin_finance":  true,
	"admin_ops":      true,
	"admin_readonly": true,
}

func main() {
	email := flag.String("email", "", "Admin email (required)")
	role := flag.String("role", "admin_readonly", "Role: admin_root | admin_finance | admin_ops | admin_readonly")
	flag.Parse()

	if *email == "" {
		fmt.Fprintln(os.Stderr, "Error: -email is required")
		flag.Usage()
		os.Exit(1)
	}
	if !validRoles[*role] {
		fmt.Fprintf(os.Stderr, "Error: invalid role %q. Valid: admin_root, admin_finance, admin_ops, admin_readonly\n", *role)
		os.Exit(1)
	}

	cfg := config.LoadConfig()
	db.Init(cfg.SupabaseURL)

	// Generate a cryptographically random 32-byte API key.
	rawKey := make([]byte, 32)
	if _, err := rand.Read(rawKey); err != nil {
		log.Fatalf("failed to generate key: %v", err)
	}
	apiKey := "exra_admin_" + hex.EncodeToString(rawKey)

	// Hash the key for DB storage.
	h := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(h[:])

	var id string
	err := db.DB.QueryRow(
		`INSERT INTO admin_users (email, role, api_key, api_key_hash, is_active)
		 VALUES ($1, $2, $3, $4, true)
		 ON CONFLICT (email) DO UPDATE
		   SET role = EXCLUDED.role,
		       api_key = EXCLUDED.api_key,
		       api_key_hash = EXCLUDED.api_key_hash,
		       is_active = true,
		       updated_at = NOW()
		 RETURNING id`,
		*email, *role, apiKey, keyHash,
	).Scan(&id)
	if err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}

	fmt.Println("✓ Admin user created/updated")
	fmt.Printf("  ID:    %s\n", id)
	fmt.Printf("  Email: %s\n", *email)
	fmt.Printf("  Role:  %s\n", *role)
	fmt.Println()
	fmt.Println("API Key (save this — shown only once):")
	fmt.Printf("  %s\n", apiKey)
	fmt.Println()
	fmt.Println("Use in requests:")
	fmt.Printf("  Authorization: Bearer %s\n", apiKey)
}
