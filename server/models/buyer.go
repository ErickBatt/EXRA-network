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
	APIKey     string    `json:"api_key,omitempty"` // only populated on CreateBuyer; omitted on lookup
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
		`SELECT id, email, balance_usd, created_at FROM buyers WHERE email = $1`,
		email,
	).Scan(&buyer.ID, &buyer.Email, &buyer.BalanceUSD, &buyer.CreatedAt)
	if err != nil {
		return nil, err
	}
	return buyer, nil
}

func GetBuyerByAPIKey(apiKey string) (*Buyer, error) {
	buyer := &Buyer{}
	// Do not SELECT api_key — the caller already has it; storing it back in Buyer
	// would risk it leaking into logs or error responses.
	err := db.DB.QueryRow(
		`SELECT id, email, balance_usd, created_at FROM buyers WHERE api_key_hash = $1`,
		hashAPIKey(apiKey),
	).Scan(&buyer.ID, &buyer.Email, &buyer.BalanceUSD, &buyer.CreatedAt)
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

// HoldBalance atomically reserves `amountUSD` on the buyer's balance
// before a Gateway session is opened. Returns ErrInsufficientBalance if the
// buyer cannot cover the hold.
//
// AUDIT §1 B3: previously CreateOfferAndMatch issued a Gateway JWT without
// checking balance, so a buyer with $0.01 could spin up N concurrent 100GB
// sessions and FinalizeSession would later fail with
// ErrInsufficientBuyerBalance leaving workers unpaid.
//
// The hold is a subtractive "balance_hold" on buyers.balance_usd, using the
// same pattern as DeductBalance (atomic SET ... WHERE balance_usd >= $1 under
// Postgres MVCC; no explicit FOR UPDATE needed because the predicate guarantees
// serialisable behaviour for this row). Release happens in ReleaseBalanceHold
// if the session never materialises or FinalizeSession settles with a smaller
// actual cost.
func HoldBalance(buyerID string, amountUSD float64) error {
	if amountUSD <= 0 {
		return nil
	}
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

// ReleaseBalanceHold returns `amountUSD` to the buyer's balance. Used by
// Gateway/Matcher clean-up paths when a held session never opens or closes
// with usage below the hold amount.
func ReleaseBalanceHold(buyerID string, amountUSD float64) error {
	if amountUSD <= 0 {
		return nil
	}
	_, err := db.DB.Exec(
		`UPDATE buyers SET balance_usd = balance_usd + $1 WHERE id = $2`,
		amountUSD, buyerID,
	)
	return err
}

func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

