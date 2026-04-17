package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractBearerTokenPrefersExraToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Exra-Token", "Exra-token")
	r.Header.Set("Authorization", "Bearer auth-token")
	r.Header.Set("X-API-Key", "api-key")

	got := extractBearerToken(r)
	if got != "Exra-token" {
		t.Fatalf("expected X-Exra-Token to be preferred, got %q", got)
	}
}

func TestExtractBearerTokenFallsBackToAuthorization(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer auth-token")
	r.Header.Set("X-API-Key", "api-key")

	got := extractBearerToken(r)
	if got != "auth-token" {
		t.Fatalf("expected bearer token, got %q", got)
	}
}

func TestExtractBearerTokenFallsBackToAPIKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "api-key")

	got := extractBearerToken(r)
	if got != "api-key" {
		t.Fatalf("expected API key fallback, got %q", got)
	}
}

func TestRoleAllowed(t *testing.T) {
	if !roleAllowed("admin_ops", []string{"admin_finance", "admin_ops"}) {
		t.Fatalf("expected role to be allowed")
	}
	if roleAllowed("admin_readonly", []string{"admin_ops"}) {
		t.Fatalf("expected role to be denied")
	}
}

func TestAdminAuthRejectsMissingToken(t *testing.T) {
	h := AdminAuth("admin_ops")(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// No Authorization header → middleware must return 401
	r := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
