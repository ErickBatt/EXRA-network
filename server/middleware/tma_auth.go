package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"exra/db"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TMASessionCookie     = "exra_tma_session"
	TMASessionTTL        = 24 * time.Hour
	TMAInitDataMaxAge    = 24 * time.Hour
	TMATelegramIDKey     = contextKey("tma_telegram_id")
	TMATelegramUserKey   = contextKey("tma_telegram_user")
	TMATelegramFirstKey  = contextKey("tma_telegram_first_name")
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

	secure := os.Getenv("TMA_COOKIE_INSECURE") != "1"
	http.SetCookie(w, &http.Cookie{
		Name:     TMASessionCookie,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
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
		if err != nil || !tok.Valid || claims.TelegramID == 0 {
			jsonError(w, "invalid tma session", http.StatusUnauthorized)
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
