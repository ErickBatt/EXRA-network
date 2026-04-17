package models

import (
	"crypto/sha256"
	"encoding/hex"
	"exra/db"
	"time"
)

type AdminUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AdminAuditLog struct {
	ID              int64     `json:"id"`
	ActorID         string    `json:"actor_id"`
	ActorEmail      string    `json:"actor_email"`
	Role            string    `json:"role"`
	Action          string    `json:"action"`
	ResourceType    string    `json:"resource_type"`
	ResourceID      string    `json:"resource_id"`
	RequestID       string    `json:"request_id"`
	IP              string    `json:"ip"`
	UserAgent       string    `json:"user_agent"`
	PayloadRedacted string    `json:"payload_redacted"`
	Result          string    `json:"result"`
	ErrorText       string    `json:"error_text"`
	CreatedAt       time.Time `json:"created_at"`
}

type AdminIncidentSummary struct {
	FailedMintQueue    int64 `json:"failed_mint_queue"`
	RetryableMintQueue int64 `json:"retryable_mint_queue"`
	PendingPayouts     int64 `json:"pending_payouts"`
	SwapGuardActive    bool  `json:"swap_guard_active"`
}

func GetAdminUserByEmail(email string) (*AdminUser, error) {
	out := &AdminUser{}
	err := db.DB.QueryRow(
		`SELECT id, email, role, is_active, created_at, updated_at
		 FROM admin_users
		 WHERE email = $1`,
		email,
	).Scan(&out.ID, &out.Email, &out.Role, &out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func GetAdminUserByAPIKey(apiKey string) (*AdminUser, error) {
	h := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(h[:])
	out := &AdminUser{}
	err := db.DB.QueryRow(
		`SELECT id, email, role, is_active, created_at, updated_at
		 FROM admin_users
		 WHERE api_key_hash = $1`,
		keyHash,
	).Scan(&out.ID, &out.Email, &out.Role, &out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func InsertAdminAuditLog(logEntry AdminAuditLog) error {
	_, err := db.DB.Exec(
		`INSERT INTO admin_audit_logs
		 (actor_id, actor_email, role, action, resource_type, resource_id, request_id, ip, user_agent, payload_redacted, result, error_text)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		logEntry.ActorID,
		logEntry.ActorEmail,
		logEntry.Role,
		logEntry.Action,
		logEntry.ResourceType,
		logEntry.ResourceID,
		logEntry.RequestID,
		logEntry.IP,
		logEntry.UserAgent,
		logEntry.PayloadRedacted,
		logEntry.Result,
		logEntry.ErrorText,
	)
	return err
}

func GetAdminIncidentSummary() (*AdminIncidentSummary, error) {
	out := &AdminIncidentSummary{}
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM oracle_mint_queue WHERE status = 'failed'`).Scan(&out.FailedMintQueue); err != nil {
		return nil, err
	}
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM oracle_mint_queue WHERE status = 'retryable'`).Scan(&out.RetryableMintQueue); err != nil {
		return nil, err
	}
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM payout_requests WHERE status = 'pending'`).Scan(&out.PendingPayouts); err != nil {
		return nil, err
	}
	paused, _ := SwapGuardState()
	out.SwapGuardActive = paused
	return out, nil
}
