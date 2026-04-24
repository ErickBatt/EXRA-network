package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func signInitData(botToken string, fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	checkString := strings.Join(parts, "\n")

	secretHmac := hmac.New(sha256.New, []byte("WebAppData"))
	secretHmac.Write([]byte(botToken))
	secret := secretHmac.Sum(nil)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(checkString))
	hash := hex.EncodeToString(mac.Sum(nil))

	v := url.Values{}
	for k, val := range fields {
		v.Set(k, val)
	}
	v.Set("hash", hash)
	return v.Encode()
}

func TestVerifyTelegramInitData_Valid(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	fields := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Unix()),
		"user":      `{"id":123,"first_name":"Bob","username":"bob"}`,
	}
	p, err := VerifyTelegramInitData(signInitData("test-token", fields))
	if err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if p.TelegramID != 123 || p.Username != "bob" {
		t.Fatalf("bad extraction: %+v", p)
	}
}

func TestVerifyTelegramInitData_ExpiredAuthDate(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	fields := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Add(-25*time.Hour).Unix()),
		"user":      `{"id":1,"first_name":"X"}`,
	}
	_, err := VerifyTelegramInitData(signInitData("test-token", fields))
	if !errors.Is(err, ErrTMAExpiredInitData) {
		t.Fatalf("expected ErrTMAExpiredInitData, got %v", err)
	}
}

func TestVerifyTelegramInitData_TamperedHash(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	fields := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Unix()),
		"user":      `{"id":1,"first_name":"X"}`,
	}
	signed := signInitData("test-token", fields)
	// Flip the last hex char of the hash.
	tampered := signed[:len(signed)-1] + "0"
	if tampered == signed {
		tampered = signed[:len(signed)-1] + "1"
	}
	_, err := VerifyTelegramInitData(tampered)
	if !errors.Is(err, ErrTMAInvalidSignature) {
		t.Fatalf("expected ErrTMAInvalidSignature, got %v", err)
	}
}

func TestVerifyTelegramInitData_Accepts30MinOld(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	fields := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Add(-30*time.Minute).Unix()),
		"user":      `{"id":42,"first_name":"Y","username":"y"}`,
	}
	if _, err := VerifyTelegramInitData(signInitData("test-token", fields)); err != nil {
		t.Fatalf("30-min-old initData should be accepted within 1h TTL, got %v", err)
	}
}

func TestVerifyTelegramInitData_Rejects90MinOld(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	fields := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Add(-90*time.Minute).Unix()),
		"user":      `{"id":1,"first_name":"Z"}`,
	}
	_, err := VerifyTelegramInitData(signInitData("test-token", fields))
	if !errors.Is(err, ErrTMAExpiredInitData) {
		t.Fatalf("90-min-old initData must be rejected (TTL=1h), got %v", err)
	}
}

func TestVerifyTelegramInitData_WrongBotToken(t *testing.T) {
	os.Setenv("TELEGRAM_BOT_TOKEN", "server-token")
	defer os.Unsetenv("TELEGRAM_BOT_TOKEN")

	fields := map[string]string{
		"auth_date": fmt.Sprintf("%d", time.Now().Unix()),
		"user":      `{"id":1,"first_name":"X"}`,
	}
	// Signed with a different token.
	signed := signInitData("attacker-token", fields)
	_, err := VerifyTelegramInitData(signed)
	if !errors.Is(err, ErrTMAInvalidSignature) {
		t.Fatalf("expected ErrTMAInvalidSignature, got %v", err)
	}
}
