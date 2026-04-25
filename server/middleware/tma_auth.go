package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"exra/db"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TMASessionCookie    = "exra_tma_session"
	TMASessionTTL       = 24 * time.Hour
	TMAInitDataMaxAge   = 1 * time.Hour // Telegram docs: reject stale initData; 1h is sufficient for replay protection
	TMATelegramIDKey    = contextKey("tma_telegram_id")
	TMATelegramUserKey  = contextKey("tma_telegram_user")
	TMATelegramFirstKey = contextKey("tma_telegram_first_name")
)

var (
	ErrTMAInvalidSignature = errors.New("invalid telegram signature")
	ErrTMAExpiredInitData  = errors.New("expired telegram initData (auth_date > 24h)")
	ErrTMAMalformedData    = errors.New("malformed telegram initData")
)

type TMAClaims struct {
	TelegramID int64  `json:"tg_id"`
	Username   string `json:"u,omitempty"`
	FirstName  string `json:"fn,omitempty"`
	jwt.RegisteredClaims
}

type TMAInitDataParams struct {
	TelegramID int64
	Username   string
	FirstName  string
	AuthDate   time.Time
}

func tmaSessionSecret() []byte {
	s := os.Getenv("TMA_SESSION_SECRET")
	if s == "" {
		s = os.Getenv("SESSION_SECRET")
	}
	if s == "" {
		// CRITICAL: In production (GO_ENV=production), this MUST be set.
		// A hardcoded fallback means any attacker can forge TMA sessions.
		if os.Getenv("GO_ENV") == "production" {
			log.Fatal("FATAL: TMA_SESSION_SECRET is not set. Set it to a random string (min 32 bytes) before starting in production.")
		}
		log.Printf("WARNING: TMA_SESSION_SECRET not set, using insecure dev fallback. DO NOT use in production!")
		s = "exra_tma_dev_secret_do_not_use_in_prod"
	}
	return []byte(s)
}

// VerifyTelegramInitData validates the HMAC signature of initData and checks
// that auth_date is within 24h. Returns extracted user params on success.
func VerifyTelegramInitData(initData string) (*TMAInitDataParams, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, errors.New("TELEGRAM_BOT_TOKEN not configured")
	}
	params, err := url.ParseQuery(initData)
	if err != nil {
		return nil, ErrTMAMalformedData
	}
	hash := params.Get("hash")
	if hash == "" {
		return nil, ErrTMAMalformedData
	}
	params.Del("hash")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params.Get(k))
	}
	checkString := strings.Join(parts, "\n")

	secretKeyHmac := hmac.New(sha256.New, []byte("WebAppData"))
	secretKeyHmac.Write([]byte(botToken))
	secretKey := secretKeyHmac.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(checkString))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return nil, ErrTMAInvalidSignature
	}

	// auth_date TTL check
	authDateStr := params.Get("auth_date")
	if authDateStr == "" {
		return nil, ErrTMAMalformedData
	}
	authDateUnix, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return nil, ErrTMAMalformedData
	}
	authDate := time.Unix(authDateUnix, 0)
	if time.Since(authDate) > TMAInitDataMaxAge {
		return nil, ErrTMAExpiredInitData
	}
	if time.Until(authDate) > 5*time.Minute {
		// future-dated clocks: reject
		return nil, ErrTMAMalformedData
	}

	userJSON := params.Get("user")
	if userJSON == "" {
		return nil, ErrTMAMalformedData
	}
	var tgUser struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
	}
	if err := json.Unmarshal([]byte(userJSON), &tgUser); err != nil || tgUser.ID == 0 {
		return nil, ErrTMAMalformedData
	}

	return &TMAInitDataParams{
		TelegramID: tgUser.ID,
		Username:   tgUser.Username,
		FirstName:  tgUser.FirstName,
		AuthDate:   authDate,
	}, nil
}

// IssueTMASession creates a JWT and sets it as HttpOnly cookie.
func IssueTMASession(w http.ResponseWriter, p *TMAInitDataParams) error {
	now := time.Now()
	claims := TMAClaims{
		TelegramID: p.TelegramID,
		Username:   p.Username,
		FirstName:  p.FirstName,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(), // jti — required for revocation (#3)
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TMASessionTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   strconv.FormatInt(p.TelegramID, 10),
			Issuer:    "exra-tma",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(tmaSessionSecret())
	if err != nil {
		return err
	}

	// TMA runs in a Telegram iframe (cross-origin). SameSite=None is required so
	// the browser sends the cookie on cross-origin fetch. Secure=true is always
	// enforced because Telegram Mini Apps are HTTPS-only.
	// Path="/" is intentional: the Next.js proxy serves /next-tma/* and the Go
	// backend serves /api/tma/* — the browser must send the cookie to both paths.
	// The JWT itself enforces auth; path restriction adds no meaningful security here.
	http.SetCookie(w, &http.Cookie{
		Name:     TMASessionCookie,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(TMASessionTTL.Seconds()),
	})
	return nil
}

// ClearTMASession removes the cookie.
func ClearTMASession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     TMASessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// RevokeTMASession extracts the jti from the request cookie and writes it to
// tma_revoked_sessions. TMAAuth will reject the token on future requests even
// if it hasn't expired yet. Safe to call on missing/invalid cookies (no-op).
func RevokeTMASession(r *http.Request) error {
	cookie, err := r.Cookie(TMASessionCookie)
	if err != nil || cookie.Value == "" {
		return nil
	}
	claims := &TMAClaims{}
	tok, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		return tmaSessionSecret(), nil
	})
	if err != nil || !tok.Valid || claims.RegisteredClaims.ID == "" {
		return nil // token already invalid — nothing to blacklist
	}
	expiresAt := time.Now().Add(TMASessionTTL)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	_, err = db.DB.Exec(
		`INSERT INTO tma_revoked_sessions(jti, expires_at) VALUES($1,$2) ON CONFLICT DO NOTHING`,
		claims.RegisteredClaims.ID, expiresAt,
	)
	return err
}

// TMAAuth middleware validates the session cookie and injects telegram_id into context.
func TMAAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(TMASessionCookie)
		if err != nil || cookie.Value == "" {
			jsonError(w, "missing tma session", http.StatusUnauthorized)
			return
		}
		claims := &TMAClaims{}
		tok, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return tmaSessionSecret(), nil
		})
		if err != nil || !tok.Valid || claims.TelegramID == 0 || claims.RegisteredClaims.ID == "" {
			jsonError(w, "invalid tma session", http.StatusUnauthorized)
			return
		}
		// #3: check revocation list — catches stolen cookies that were explicitly logged out.
		var revoked bool
		if err := db.DB.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM tma_revoked_sessions WHERE jti=$1 AND expires_at > NOW())`,
			claims.RegisteredClaims.ID,
		).Scan(&revoked); err != nil || revoked {
			jsonError(w, "session revoked", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), TMATelegramIDKey, claims.TelegramID)
		ctx = context.WithValue(ctx, TMATelegramUserKey, claims.Username)
		ctx = context.WithValue(ctx, TMATelegramFirstKey, claims.FirstName)
		next(w, r.WithContext(ctx))
	}
}

// TelegramIDFromContext extracts tg_id set by TMAAuth middleware.
func TelegramIDFromContext(r *http.Request) int64 {
	v, _ := r.Context().Value(TMATelegramIDKey).(int64)
	return v
}

// TelegramUsernameFromContext returns the username captured at session issue time.
func TelegramUsernameFromContext(r *http.Request) string {
	v, _ := r.Context().Value(TMATelegramUserKey).(string)
	return v
}

// TelegramFirstNameFromContext returns the first name captured at session issue time.
func TelegramFirstNameFromContext(r *http.Request) string {
	v, _ := r.Context().Value(TMATelegramFirstKey).(string)
	return v
}

// AssertDeviceOwnedByTelegram verifies that deviceID is linked to tgID with status='linked'.
// Returns true if owned. Callers should 403 on false.
func AssertDeviceOwnedByTelegram(tgID int64, deviceID string) (bool, error) {
	if tgID == 0 || deviceID == "" {
		return false, nil
	}
	var ok bool
	err := db.DB.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM tma_devices WHERE telegram_id=$1 AND device_id=$2 AND status='linked')`,
		tgID, deviceID,
	).Scan(&ok)
	return ok, err
}
