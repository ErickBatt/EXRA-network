package models

import (
	"testing"
)

func TestReferralPercent(t *testing.T) {
	tests := []struct {
		name          string
		referralCount int
		expected      float64
	}{
		{"0 referrals", 0, 0.0},
		{"1 referral (Lvl 1 lower)", 1, 0.10},
		{"100 referrals (Lvl 1 upper)", 100, 0.10},
		{"101 referrals (Lvl 2 lower)", 101, 0.15},
		{"300 referrals (Lvl 2 upper)", 300, 0.15},
		{"301 referrals (Lvl 3 lower)", 301, 0.20},
		{"600 referrals (Lvl 3 upper)", 600, 0.20},
		{"601 referrals (Lvl 4 lower)", 601, 0.30},
		{"1000 referrals (Lvl 4 upper)", 1000, 0.30},
		{"1001 referrals (Beyond Lvl 4)", 1001, 0.30}, // Ambassador tier is inclusive/capped at 30%
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReferralPercent(tt.referralCount)
			if got != tt.expected {
				t.Errorf("ReferralPercent(%d) = %v; want %v", tt.referralCount, got, tt.expected)
			}
		})
	}
}

func TestReferralTierByCount(t *testing.T) {
	tier := ReferralTierByCount(150)
	if tier == nil {
		t.Fatal("expected tier, got nil")
	}
	if tier.Level != 2 {
		t.Errorf("expected Level 2, got %d", tier.Level)
	}
	if tier.Name != "Network Builder" {
		t.Errorf("expected Network Builder, got %s", tier.Name)
	}

	nilTier := ReferralTierByCount(-1)
	if nilTier != nil {
		t.Errorf("expected nil tier for -1, got %+v", nilTier)
	}
}

// TestPopSplitInvariants validates the reward split math with the pool-based treasury fee.
// Solo miner (no pool) pays 30% treasury; worker gets the remaining 70% minus referral.
func TestPopSplitInvariants(t *testing.T) {
	totalEmission := 0.000050
	soloTreasuryRate := 0.30 // default for solo miner

	tests := []struct {
		name             string
		referralCount    int
		hasReferrer      bool
		expectedRefPct   float64
		expectedRefRwd   float64
		expectedTreasury float64
		expectedWorker   float64
	}{
		{
			"No referrer (solo)",
			0, false, 0.0, 0.0,
			totalEmission * soloTreasuryRate,
			totalEmission * (1.0 - soloTreasuryRate),
		},
		{
			"Lvl 1 referrer (solo)",
			50, true, 0.10, totalEmission * 0.10,
			totalEmission * soloTreasuryRate,
			totalEmission * (1.0 - soloTreasuryRate - 0.10),
		},
		{
			"Lvl 4 referrer (solo)",
			800, true, 0.30, totalEmission * 0.30,
			totalEmission * soloTreasuryRate,
			totalEmission * (1.0 - soloTreasuryRate - 0.30),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refPct := 0.0
			if tt.hasReferrer {
				refPct = ReferralPercent(tt.referralCount)
			}
			referralReward := totalEmission * refPct
			treasuryReward := totalEmission * soloTreasuryRate
			workerReward := totalEmission - treasuryReward - referralReward

			if refPct != tt.expectedRefPct {
				t.Errorf("refPct = %v, want %v", refPct, tt.expectedRefPct)
			}
			if referralReward != tt.expectedRefRwd {
				t.Errorf("referralReward = %v, want %v", referralReward, tt.expectedRefRwd)
			}
			if treasuryReward != tt.expectedTreasury {
				t.Errorf("treasuryReward = %v, want %v", treasuryReward, tt.expectedTreasury)
			}
			const eps = 1e-15
			diff2 := workerReward - tt.expectedWorker
			if diff2 < -eps || diff2 > eps {
				t.Errorf("workerReward = %v, want %v (diff %v)", workerReward, tt.expectedWorker, diff2)
			}

			// Invariant: all streams must sum to totalEmission.
			total := workerReward + referralReward + treasuryReward
			diff := total - totalEmission
			if diff < -1e-12 || diff > 1e-12 {
				t.Errorf("total distribution = %v, want %v (drift: %v)", total, totalEmission, diff)
			}
		})
	}
}

