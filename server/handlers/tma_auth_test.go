package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// buildSignedInitData forges a Telegram-style initData query string signed
// with the given bot token so tests can exercise the real verifier end-to-end.
func buildSignedInitData(t *testing.T, botToken string, authDate time.Time, telegramID int64) string {
	t.Helper()
	userJSON := fmt.Sprintf(`{"id":%d,"first_name":"Alice","username":"alice"}`, telegramID)
	params := url.Values{}
	params.Set("auth_date", fmt.Sprintf("%d", authDate.Unix()))
	params.Set("user", userJSON)
	params.Set("query_id", "test-query")

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
	sig := hex.EncodeToString(mac.Sum(nil))

	params.Set("hash", sig)
	return params.Encode()
}

func withBotToken(t *testing.T, token string) func() {
	t.Helper()
	old := os.Getenv("TELEGRAM_BOT_TOKEN")
	os.Setenv("TELEGRAM_BOT_TOKEN", token)
	return func() { os.Setenv("TELEGRAM_BOT_TOKEN", old) }
}

func TestVerifyTelegramInitData_AcceptsFreshSignedData(t *testing.T) {
	restore := withBotToken(t, "test-bot-token-fresh")
	defer restore()

	initData := buildSignedInitData(t, "test-bot-token-fresh", time.Now(), 12345)
	ident, err := telegramUserFromInitData(initData)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if ident.ID != 12345 {
		t.Fatalf("expected telegram id 12345, got %d", ident.ID)
	}
}

func TestVerifyTelegramInitData_RejectsTamperedHash(t *testing.T) {
	restore := withBotToken(t, "test-bot-token-tampered")
	defer restore()

	initData := buildSignedInitData(t, "test-bot-token-tampered", time.Now(), 777)
	// Flip the last hash character so the HMAC no longer verifies.
	tampered := strings.Replace(initData, "hash=", "hash=0", 1)
	_, err := telegramUserFromInitData(tampered)
	if !errors.Is(err, errInitDataInvalidSig) {
		t.Fatalf("expected invalid sig error, got %v", err)
	}
}

func TestVerifyTelegramInitData_RejectsWrongBotToken(t *testing.T) {
	restore := withBotToken(t, "server-token")
	defer restore()

	// Sign with a different token than the server knows — attacker-generated.
	initData := buildSignedInitData(t, "attacker-token", time.Now(), 99)
	_, err := telegramUserFromInitData(initData)
	if !errors.Is(err, errInitDataInvalidSig) {
		t.Fatalf("expected invalid sig error, got %v", err)
	}
}

func TestVerifyTelegramInitData_RejectsExpiredAuthDate(t *testing.T) {
	restore := withBotToken(t, "token-expired")
	defer restore()

	// 48h-old initData should be past the 24h freshness window.
	initData := buildSignedInitData(t, "token-expired", time.Now().Add(-48*time.Hour), 1)
	_, err := telegramUserFromInitData(initData)
	if !errors.Is(err, errInitDataExpired) {
		t.Fatalf("expected expired error, got %v", err)
	}
}

// TmaLinkDevice must refuse a request where no init_data is provided even if
// the caller supplies a telegram_id directly — that was the pre-fix attack.
func TestTmaLinkDevice_RejectsMissingInitData(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"device_id":   "dev-abc",
		"telegram_id": 999, // attacker-controlled, must be ignored
	})
	req := httptest.NewRequest("POST", "/api/tma/link-device", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	TmaLinkDevice(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "init_data") {
		t.Fatalf("expected init_data in error, got %s", rr.Body.String())
	}
}

func TestTmaLinkDevice_RejectsInvalidInitData(t *testing.T) {
	restore := withBotToken(t, "any-token")
	defer restore()

	body, _ := json.Marshal(map[string]any{
		"init_data": "user=%7B%22id%22%3A1%7D&auth_date=1&hash=deadbeef",
		"device_id": "dev-abc",
	})
	req := httptest.NewRequest("POST", "/api/tma/link-device", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	TmaLinkDevice(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}
