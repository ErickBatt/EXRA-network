package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGearScoreV2_FeederTrustWeight(t *testing.T) {
	// Formula: 0.4*HW + 0.3*Uptime + 0.2*Quality + 0.1*(Feeder_Trust*1000)
	hw := 1000
	uptime := 1000
	quality := 1000
	
	// With feeder trust 1.0 -> 0.1 * 1000 = 100
	gsMax := ComputeGearScoreV2(hw, uptime, quality, 1.0)
	assert.Equal(t, 1000, gsMax.Score)

	// With feeder trust 0.0 -> 0 
	// Max score possible = 400 + 300 + 200 + 0 = 900
	gsMin := ComputeGearScoreV2(hw, uptime, quality, 0.0)
	assert.Equal(t, 900, gsMin.Score)

	// With feeder trust 0.5 -> 0.1 * 500 = 50
	gsMid := ComputeGearScoreV2(hw, uptime, quality, 0.5)
	assert.Equal(t, 950, gsMid.Score)
}

func TestRSMultiplierAnonCap(t *testing.T) {
	// Anon cap means even with GS = 1000, RS_mult = 0.5 (max 0.5x)
	multMax := ComputeRSMultiplier(1000, "anon")
	assert.Equal(t, 0.5, multMax, "Anon should not exceed 0.5x RS multiplier")

	multMin := ComputeRSMultiplier(0, "anon")
	assert.Equal(t, 0.5, multMin, "Anon should not drop below 0.5x RS multiplier")
}

func TestRSMultiplierPeakMax(t *testing.T) {
	// Peak tier allows up to 2.0x (1000 / 500)
	multHigh := ComputeRSMultiplier(1000, "peak")
	assert.Equal(t, 2.0, multHigh, "Peak should reach 2.0x for max GS")

	multMid := ComputeRSMultiplier(750, "peak")
	assert.Equal(t, 1.5, multMid, "Peak intermediate score scales properly")

	multLow := ComputeRSMultiplier(0, "peak")
	assert.Equal(t, 0.5, multLow, "Peak should not drop below 0.5x minimum")
}

// Ensure the older `ComputeGearScore` returns the right types to be fed into V2
func TestGearScoreLegacyCompat(t *testing.T) {
	hwGs := ComputeGearScore("Workstation", 16, 16384, 32768, 1000, true)
	assert.Greater(t, hwGs.Score, 500)

	gsV2 := ComputeGearScoreV2(hwGs.Score, 990, 1000, 0.5)
	assert.Greater(t, gsV2.Score, 500)
}

func TestGearScoreDecay(t *testing.T) {
	// Pending actual implementation of Uptime tracking in database, 
	// the decay algorithm isn't fully active yet, but we stub this test
	// to document the expected behavior of Uptime score reduction.
	// E.g., GS = 0.4*HW + 0.3*Uptime(99%->990) + ...
	// If a node is inactive, its uptime drops, reducing final score.
	
	// Active node: uptime = 1000
	activeGS := ComputeGearScoreV2(1000, 1000, 1000, 1.0)
	assert.Equal(t, 1000, activeGS.Score)

	// Inactive node (e.g. absent for a week, Uptime drops to 660, yielding 66% uptime)
	// Decayed by ~10% of total score (1000 -> 900)
	// That corresponds to Uptime losing ~333 points (0.3 * 333 = ~100 points dropped)
	decayedGS := ComputeGearScoreV2(1000, 666, 1000, 1.0)
	assert.InDelta(t, 900, decayedGS.Score, 1.0, "10%% weekly decay should be possible by dropping Uptime score")
}
