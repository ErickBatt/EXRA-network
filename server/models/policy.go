package models

import (
	"math"
)

// EmissionPolicy defines the EXRA tokenomics parameters.
type EmissionPolicy struct {
	MaxSupply float64
	EpochSize float64
	Finalized bool
}

var currentPolicy = EmissionPolicy{
	MaxSupply: 1_000_000_000,
	EpochSize: 100_000_000,
	Finalized: true,
}

func SetPolicy(p EmissionPolicy) {
	currentPolicy = p
}

func GetPolicy() EmissionPolicy {
	return currentPolicy
}

type EpochState struct {
	EpochIndex     int     `json:"epoch_index"`
	Name           string  `json:"name"`
	ProgressPct    float64 `json:"progress_pct"`
	RemainingExra  float64 `json:"remaining_exra"`
	DaysRemaining  float64 `json:"days_remaining,omitempty"`
}

func CalculateEpochMultiplier(currentSupply float64) float64 {
	// Epoch 1 (0..100M): 1.0x
	// Epoch 2 (100M..200M): 0.5x
	// Epoch 3 (200M..300M): 0.25x
	// ...
	epochNum := int(currentSupply / currentPolicy.EpochSize)
	if epochNum < 0 {
		return 1.0
	}
	// Multiplier = 1 / 2^epochNum
	return math.Pow(0.5, float64(epochNum))
}

func CheckCurrentEpoch(mintedTotal float64) EpochState {
	return CheckCurrentEpochWithRate(mintedTotal, 0)
}

func CheckCurrentEpochWithRate(mintedTotal float64, dailyRate float64) EpochState {
	epochNum := int(mintedTotal / currentPolicy.EpochSize) + 1
	progress := mintedTotal - float64(epochNum-1)*currentPolicy.EpochSize
	progressPct := (progress / currentPolicy.EpochSize) * 100
	remaining := currentPolicy.EpochSize - progress

	state := EpochState{
		EpochIndex:    epochNum,
		Name:          "Epoch " + string(rune('0'+epochNum)), // Epoch 1, 2...
		ProgressPct:   progressPct,
		RemainingExra: remaining,
	}
	
	if epochNum > 9 {
		state.Name = "Epoch final"
	}

	if dailyRate > 0 {
		state.DaysRemaining = math.Floor(remaining / dailyRate)
	}
	
	return state
}
