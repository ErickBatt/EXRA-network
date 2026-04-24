package models

// node_test.go — source-level regressions for node model security.

import (
	"os"
	"strings"
	"testing"
)

// TestSetNodeOffline_PausesListingsInSource guards Fix #6: when a device
// disconnects, its active marketplace listings must be paused atomically.
// Buyers must not see listings for offline devices.
func TestSetNodeOffline_PausesListingsInSource(t *testing.T) {
	src, err := os.ReadFile("node.go")
	if err != nil {
		t.Fatalf("cannot read node.go: %v", err)
	}
	text := string(src)

	start := strings.Index(text, "func SetNodeOfflineByDeviceID(")
	if start == -1 {
		t.Fatal("SetNodeOfflineByDeviceID not found in node.go")
	}
	// Take enough bytes to cover the function body.
	end := start + 800
	if end > len(text) {
		end = len(text)
	}
	body := text[start:end]

	if !strings.Contains(body, "worker_listings") {
		t.Fatal("Fix #6 regression: SetNodeOfflineByDeviceID does not pause worker_listings")
	}
	if !strings.Contains(body, "status = 'paused'") {
		t.Fatal("Fix #6 regression: SetNodeOfflineByDeviceID does not set listings to paused")
	}
	if !strings.Contains(body, "tx.Commit()") {
		t.Fatal("Fix #6 regression: SetNodeOfflineByDeviceID must commit both updates atomically")
	}
}
