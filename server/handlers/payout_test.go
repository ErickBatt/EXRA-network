package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// withMuxVars invokes the handler through a router so mux.Vars() works.
func withMuxVars(method, path, route string, body []byte, headers map[string]string, h http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBuffer(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r := mux.NewRouter()
	r.HandleFunc(route, h).Methods(method)
	r.ServeHTTP(rr, req)
	return rr
}

// Covers the defense-in-depth check inside ClaimPayoutHandler:
// even if DIDAuth is misconfigured, the handler must refuse a call where
// the X-DID header does not match the DID in the URL path.
func TestClaimPayoutHandler_RejectsMismatchedDID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"amount_usd":       1.0,
		"recipient_wallet": "0xattacker",
	})
	rr := withMuxVars(
		"POST",
		"/claim/did:peaq:victim",
		"/claim/{did}",
		body,
		map[string]string{"X-DID": "did:peaq:attacker"},
		ClaimPayoutHandler,
	)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "does not match")
}

func TestClaimPayoutHandler_RejectsMissingSignedDID(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"amount_usd":       1.0,
		"recipient_wallet": "0xwallet",
	})
	rr := withMuxVars(
		"POST",
		"/claim/did:peaq:victim",
		"/claim/{did}",
		body,
		nil,
		ClaimPayoutHandler,
	)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestClaimPayoutHandler_RejectsInvalidAmount(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"amount_usd":       -5.0,
		"recipient_wallet": "0xwallet",
	})
	rr := withMuxVars(
		"POST",
		"/claim/did:peaq:abc",
		"/claim/{did}",
		body,
		map[string]string{"X-DID": "did:peaq:abc"},
		ClaimPayoutHandler,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "amount_usd")
}

func TestClaimPayoutHandler_RejectsEmptyWallet(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"amount_usd":       1.0,
		"recipient_wallet": "",
	})
	rr := withMuxVars(
		"POST",
		"/claim/did:peaq:abc",
		"/claim/{did}",
		body,
		map[string]string{"X-DID": "did:peaq:abc"},
		ClaimPayoutHandler,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "recipient_wallet")
}
