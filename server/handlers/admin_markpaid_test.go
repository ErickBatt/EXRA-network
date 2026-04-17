package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdminMarkPayoutPaid_RejectsMissingTxHash(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"provider": "stripe"})
	rr := withMuxVars(
		"POST",
		"/api/admin/payout/p1/mark-paid",
		"/api/admin/payout/{id}/mark-paid",
		body,
		nil,
		AdminMarkPayoutPaid,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "tx_hash")
}

func TestAdminMarkPayoutPaid_RejectsInvalidJSON(t *testing.T) {
	rr := withMuxVars(
		"POST",
		"/api/admin/payout/p1/mark-paid",
		"/api/admin/payout/{id}/mark-paid",
		[]byte("{not json"),
		nil,
		AdminMarkPayoutPaid,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminMarkPayoutPaid_RejectsOversizedField(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"tx_hash": strings.Repeat("a", 257),
	})
	rr := withMuxVars(
		"POST",
		"/api/admin/payout/p1/mark-paid",
		"/api/admin/payout/{id}/mark-paid",
		body,
		nil,
		AdminMarkPayoutPaid,
	)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "maximum length")
}
