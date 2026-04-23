package hub

// client_trust_test.go keeps lightweight source-level regressions for the
// trust fixes that used to be red findings in the marketplace audit.

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

func extractCaseBody(t *testing.T, filePath, caseName string) string {
	t.Helper()

	src, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", filePath, err)
	}

	re := regexp.MustCompile(fmt.Sprintf(`(?s)case %q:(.*?)(?:case "|[\t ]+}\s*}\s*$)`, caseName))
	matches := re.FindSubmatch(src)
	if matches == nil {
		t.Fatalf("cannot find case %q in %s", caseName, filePath)
	}
	return string(matches[1])
}

func TestCanary_UsesPerTaskHashInSource(t *testing.T) {
	src, err := os.ReadFile("../models/fraud.go")
	if err != nil {
		t.Fatalf("cannot read fraud.go: %v", err)
	}

	if regexp.MustCompile(`expectedResult\s*:=\s*"canary_expected_hash"`).Match(src) {
		t.Fatal("E2 regression: fraud.go still contains the old universal canary literal")
	}

	text := string(src)
	for _, marker := range []string{
		`crand.Read(`,
		`sha256.Sum256(`,
		`expectedResult := hex.EncodeToString`,
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("expected fraud.go to contain %q for per-task canary generation", marker)
		}
	}
}

func TestFeederReport_RequiresSignatureInSource(t *testing.T) {
	caseBody := extractCaseBody(t, "client.go", "feeder_report")

	for _, marker := range []string{
		`msg.Data.Signature`,
		`VerifyDIDSignature`,
		`fmt.Sprintf("%d:%s:%s"`,
	} {
		if !strings.Contains(caseBody, marker) {
			t.Fatalf("E1 regression: feeder_report is missing %q", marker)
		}
	}
}

func TestTrafficReport_ClampsUntrustedBytesInSource(t *testing.T) {
	caseBody := extractCaseBody(t, "client.go", "traffic")

	for _, marker := range []string{
		`MaxTrafficPerSec`,
		`if msg.Bytes > MaxTrafficPerSec`,
		`AddNodeTrafficByDeviceID`,
	} {
		if !strings.Contains(caseBody, marker) {
			t.Fatalf("expected traffic hardening marker %q in traffic case", marker)
		}
	}

	sessionSrc, err := os.ReadFile("../models/session.go")
	if err == nil && !strings.Contains(string(sessionSrc), "buyer_reported") {
		t.Log("buyer-side cross-check is still a documented follow-up; current test only guards the server-side clamp")
	}
}
