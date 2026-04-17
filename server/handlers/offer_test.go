package handlers

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// CreateOffer must reject malformed bodies BEFORE looking at the buyer
// context, otherwise unauthenticated callers cause a nil-pointer panic.
// All these cases run without a buyer in context — they should still
// surface a clean 400, not a 500/panic.

func postOffer(body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/offers", bytes.NewBuffer(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	CreateOffer(rr, req)
	return rr
}

func TestCreateOffer_RejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/offers", strings.NewReader("{not json"))
	rr := httptest.NewRecorder()
	CreateOffer(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateOffer_RejectsMissingCountry(t *testing.T) {
	rr := postOffer(map[string]any{
		"target_gb":         10.0,
		"max_price_per_gb":  1.5,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "country")
}

func TestCreateOffer_RejectsCountryWithDigits(t *testing.T) {
	rr := postOffer(map[string]any{
		"country":          "U5",
		"target_gb":        10.0,
		"max_price_per_gb": 1.5,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "country")
}

func TestCreateOffer_RejectsOversizedCountry(t *testing.T) {
	rr := postOffer(map[string]any{
		"country":          strings.Repeat("A", 9),
		"target_gb":        10.0,
		"max_price_per_gb": 1.5,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "country")
}

func TestCreateOffer_RejectsZeroTargetGB(t *testing.T) {
	rr := postOffer(map[string]any{
		"country":          "IN",
		"target_gb":        0.0,
		"max_price_per_gb": 1.5,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "target_gb")
}

func TestCreateOffer_RejectsNegativePrice(t *testing.T) {
	rr := postOffer(map[string]any{
		"country":          "IN",
		"target_gb":        10.0,
		"max_price_per_gb": -1.0,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "max_price_per_gb")
}

func TestCreateOffer_RejectsOversizedTargetGB(t *testing.T) {
	rr := postOffer(map[string]any{
		"country":          "IN",
		"target_gb":        100_001.0,
		"max_price_per_gb": 1.5,
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "target_gb")
}

// The legacy `<= 0` guard let NaN through (NaN <= 0 is false in Go).
// validateFloat closes that hole — verify.
func TestCreateOffer_RejectsNaNTargetGB(t *testing.T) {
	// json.Marshal can't encode NaN, so build the body by hand.
	body := []byte(`{"country":"IN","target_gb":` + jsonNumberFor(math.NaN()) + `,"max_price_per_gb":1.5}`)
	req := httptest.NewRequest("POST", "/api/offers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	CreateOffer(rr, req)
	// json.Decode itself rejects literal NaN; we accept either path as long
	// as it's a clean 400.
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// jsonNumberFor returns a literal token the offer handler will see as a
// float; for NaN/Inf the JSON spec disallows them, so json.Decode 400s
// before validateFloat — which is exactly the safe outcome we want.
func jsonNumberFor(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	default:
		return "0"
	}
}
