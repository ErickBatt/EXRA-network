package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// In the Peaq V2 fee model BuildPayoutPrecheck treats PayoutFeeBreakdown
// values as already-in-USD (the legacy TON-priced multiplier is gone), so
// these tests assert the simplified arithmetic directly.

func TestBuildPayoutPrecheck_BlockNegativeTransfer(t *testing.T) {
	// Fees ($0.21) exceed gross amount ($0.10) — net is negative and
	// CanPayout must be false with the user-facing alert populated.
	fees := PayoutFeeBreakdown{
		GasFeeChain:     0.05,
		StorageFeeChain: 0.16,
		TotalFeeChain:   0.21,
	}
	pre := BuildPayoutPrecheck("dev-1", "wallet-1", 0.10, 10.0, fees)

	assert.False(t, pre.CanPayout)
	assert.Less(t, pre.NetAmountUSD, 0.0)
	assert.Equal(t, "Баланса недостаточно для оплаты газа", pre.Alert)
}

func TestBuildPayoutPrecheck_GasCalculationInvariance(t *testing.T) {
	// Gross $50.00, total fee $1.00 (already USD) -> net $49.00.
	fees := PayoutFeeBreakdown{
		GasFeeChain:   1.0,
		TotalFeeChain: 1.0,
	}
	pre := BuildPayoutPrecheck("dev-1", "wallet-1", 50.0, 100.0, fees)

	assert.True(t, pre.CanPayout)
	assert.Equal(t, 1.0, pre.TotalFeeUSD)
	assert.Equal(t, 49.0, pre.NetAmountUSD)
	assert.Empty(t, pre.Alert)
}
