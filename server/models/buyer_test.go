package models

// buyer_test.go — source-level regressions for buyer model security.
// DB-free: reads source text to guard against re-introduction of raw key leaks.

import (
	"os"
	"strings"
	"testing"
)

// TestGetBuyerByEmail_NoAPIKeyInSelect guards that GetBuyerByEmail never
// fetches the raw api_key column from the DB. Raw keys must only be returned
// by CreateBuyer (one-time issuance). Leaking them via lookup functions risks
// accidental exposure in logs and API responses.
func TestGetBuyerByEmail_NoAPIKeyInSelect(t *testing.T) {
	src, err := os.ReadFile("buyer.go")
	if err != nil {
		t.Fatalf("cannot read buyer.go: %v", err)
	}
	text := string(src)

	// Locate the GetBuyerByEmail function body heuristically.
	start := strings.Index(text, "func GetBuyerByEmail(")
	if start == -1 {
		t.Fatal("GetBuyerByEmail not found in buyer.go")
	}
	// Take the next 400 bytes — enough to cover the function body.
	end := start + 400
	if end > len(text) {
		end = len(text)
	}
	body := text[start:end]

	if strings.Contains(body, "api_key,") || strings.Contains(body, ", api_key") || strings.Contains(body, "SELECT id, api_key") {
		t.Fatal("Fix #3 regression: GetBuyerByEmail selects api_key from DB — raw keys must never be fetched on lookup")
	}
	if strings.Contains(body, "&buyer.APIKey") {
		t.Fatal("Fix #3 regression: GetBuyerByEmail scans into buyer.APIKey — raw keys must never be populated on lookup")
	}
}

// TestBuyerStruct_APIKeyOmitEmpty ensures the APIKey JSON tag has omitempty so
// it is absent from all lookup responses where the field is not populated.
func TestBuyerStruct_APIKeyOmitEmpty(t *testing.T) {
	src, err := os.ReadFile("buyer.go")
	if err != nil {
		t.Fatalf("cannot read buyer.go: %v", err)
	}
	if !strings.Contains(string(src), `json:"api_key,omitempty"`) {
		t.Fatal("Buyer.APIKey must have omitempty JSON tag to prevent empty string leaking into responses")
	}
}
