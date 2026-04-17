package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"exra/db"
	"time"
)

// hashAPIKey returns the SHA-256 hex digest of an API key.
// Used for secure DB lookup — raw keys are never stored for auth purposes.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

type Buyer struct {
	ID         string    `json:"id"`
	APIKey     string    `json:"api_key"`
	Email      string    `json:"email"`
	BalanceUSD float64   `json:"balance_usd"`
	CreatedAt  time.Time `json:"created_at"`
}

var ErrInsufficientBalance = errors.New("insufficient balance")

func CreateBuyer(email string) (*Buyer, error) {
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	keyHash := hashAPIKey(apiKey)
	buyer := &Buyer{}
	err = db.DB.QueryRow(
		`INSERT INTO buyers (api_key, api_key_hash, email) VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id, api_key, email, balance_usd, created_at`,
		apiKey, keyHash, email,
	).Scan(&buyer.ID, &buyer.APIKey, &buyer.Email, &buyer.BalanceUSD, &buyer.CreatedAt)
	return buyer, err
}

func GetBuyerByEmail(email string) (*Buyer, error) {
	buyer := &Buyer{}
	err := db.DB.QueryRow(
		`SELECT id, api_key, email, balance_usd, created_at FROM buyers WHERE email = $1`,
		email,
	).Scan(&buyer.ID, &buyer.APIKey, &buyer.Email, &buyer.BalanceUSD, &buyer.CreatedAt)
	if err != nil {
		return nil, err
	}
	return buyer, nil
}

func GetBuyerByAPIKey(apiKey string) (*Buyer, error) {
	buyer := &Buyer{}
	err := db.DB.QueryRow(
		`SELECT id, api_key, email, balance_usd, created_at FROM buyers WHERE api_key_hash = $1`,
		hashAPIKey(apiKey),
	).Scan(&buyer.ID, &buyer.APIKey, &buyer.Email, &buyer.BalanceUSD, &buyer.CreatedAt)
	if err != nil {
		return nil, err
	}
	return buyer, nil
}

func TopUpBalance(buyerID string, amountUSD float64) error {
	_, err := db.DB.Exec(
		`UPDATE buyers SET balance_usd = balance_usd + $1 WHERE id = $2`,
		amountUSD, buyerID,
	)
	return err
}

func DeductBalance(buyerID string, amountUSD float64) error {
	res, err := db.DB.Exec(
		`UPDATE buyers
		 SET balance_usd = balance_usd - $1
		 WHERE id = $2 AND balance_usd >= $1`,
		amountUSD, buyerID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInsufficientBalance
	}
	return nil
}

func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

