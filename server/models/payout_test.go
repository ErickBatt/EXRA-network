package models

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestBuildPayoutPrecheck_BlockNegativeTransfer(t *testing.T) {
	// Scenario: User has $0.10 earnings, but fees are $0.15.
	// NetAmount should be negative, CanPayout should be false.
	fees := PayoutFeeBreakdown{
		GasFeeChain:     0.01,
		StorageFeeChain: 0.02, // Total fee ≈ $0.21 if Chain is $7.0
		TotalFeeChain:   0.03,
	}
	pre := BuildPayoutPrecheck("dev-1", "wallet-1", 0.10, 10.0, 7.0, fees)
	
	assert.False(t, pre.CanPayout)
	assert.Less(t, pre.NetAmountUSD, 0.0)
	assert.Equal(t, "Баланса недостаточно для оплаты газа", pre.Alert)
}

func TestBuildPayoutPrecheck_GasCalculationInvariance(t *testing.T) {
	// Scenario: Check math accuracy.
	// Gross: $50.00, Fee: 0.1 TON, Price: $10 -> Total Fee: $1.00 -> Net: $49.00
	fees := PayoutFeeBreakdown{
		GasFeeChain:   0.1,
		TotalFeeChain: 0.1,
	}
	pre := BuildPayoutPrecheck("dev-1", "wallet-1", 50.0, 100.0, 10.0, fees)
	
	assert.True(t, pre.CanPayout)
	assert.Equal(t, 1.0, pre.TotalFeeUSD)
	assert.Equal(t, 49.0, pre.NetAmountUSD)
}
