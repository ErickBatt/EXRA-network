package models

import (
	"math"
	"strings"
)

// GearGrade represents the node performance tier.
// D → C → B → A → S, each giving higher reward multiplier and
// priority in oracle task assignment.
type GearGrade string

const (
	GradeD GearGrade = "D" // Basic phone, minimal bandwidth
	GradeC GearGrade = "C" // Mid-range phone or old PC
	GradeB GearGrade = "B" // Good phone or mid PC
	GradeA GearGrade = "A" // High-end phone or gaming PC
	GradeS GearGrade = "S" // Workstation / server-grade
)

// GearScore holds a node's computed grade and reward multiplier.
type GearScore struct {
	Grade      GearGrade `json:"grade"`
	Score      int       `json:"score"`       // 0–1000 raw score
	Multiplier float64   `json:"multiplier"`  // Applied to PoP and usage rewards
	Tier       string    `json:"tier"`        // Human label: "Standard", "Pro", etc.
}

// gradeThresholds defines the minimum score for each grade.
var gradeThresholds = []struct {
	MinScore   int
	Grade      GearGrade
	Multiplier float64
	Tier       string
}{
	{800, GradeS, 2.00, "Elite"},
	{600, GradeA, 1.50, "Pro"},
	{400, GradeB, 1.00, "Standard"},
	{200, GradeC, 0.75, "Basic"},
	{0,   GradeD, 0.50, "Entry"},
}

// ComputeGearScore calculates a node's Gear Score from its hardware profile.
//
// Scoring factors:
//   - Device type / architecture  (0–200 pts)
//   - RAM                         (0–200 pts)
//   - VRAM / GPU presence         (0–200 pts)
//   - Bandwidth                   (0–200 pts)
//   - Residential IP bonus         (0–100 pts)
//   - CPU cores                   (0–100 pts)
func ComputeGearScore(deviceType string, cpuCores, vramMB, ramMB, bandwidthMbps int, isResidential bool) GearScore {
	score := 0

	// --- Device type (0–200 pts) ---
	dt := strings.ToLower(deviceType)
	switch {
	case strings.Contains(dt, "server") || strings.Contains(dt, "workstation"):
		score += 200
	case strings.Contains(dt, "x86") || strings.Contains(dt, "pc") || strings.Contains(dt, "desktop"):
		score += 160
	case strings.Contains(dt, "laptop") || strings.Contains(dt, "notebook"):
		score += 120
	case strings.Contains(dt, "tablet"):
		score += 80
	case strings.Contains(dt, "phone") || strings.Contains(dt, "android") || strings.Contains(dt, "mobile"):
		score += 60
	default:
		score += 40
	}

	// --- RAM (0–200 pts) ---
	// 1GB → 20pts, 4GB → 80pts, 8GB → 130pts, 16GB → 170pts, 32GB+ → 200pts
	ramGB := ramMB / 1024
	ramScore := int(math.Min(200, float64(ramGB)*6.25))
	score += ramScore

	// --- VRAM / GPU (0–200 pts) ---
	// No GPU → 0, 2GB → 50, 4GB → 90, 8GB → 140, 16GB+ → 200
	switch {
	case vramMB >= 16*1024:
		score += 200
	case vramMB >= 8*1024:
		score += 140
	case vramMB >= 4*1024:
		score += 90
	case vramMB >= 2*1024:
		score += 50
	case vramMB > 0:
		score += 20
	}

	// --- Bandwidth (0–200 pts) ---
	// 1 Mbps → 5pts, 10 Mbps → 50pts, 50 Mbps → 100pts, 200 Mbps → 160pts, 1Gbps+ → 200pts
	bwScore := int(math.Min(200, float64(bandwidthMbps)*0.2))
	score += bwScore

	// --- Residential IP bonus (0–100 pts) ---
	// Residential IPs are significantly more valuable to buyers (harder to block)
	if isResidential {
		score += 100
	}

	// --- CPU cores (0–100 pts) ---
	// 1 core → 10pts, 4 cores → 40pts, 8 cores → 70pts, 16+ cores → 100pts
	coreScore := int(math.Min(100, float64(cpuCores)*6.25))
	score += coreScore

	// Cap at 1000
	if score > 1000 {
		score = 1000
	}

	// Map score to grade
	for _, t := range gradeThresholds {
		if score >= t.MinScore {
			return GearScore{
				Grade:      t.Grade,
				Score:      score,
				Multiplier: t.Multiplier,
				Tier:       t.Tier,
			}
		}
	}

	return GearScore{Grade: GradeD, Score: score, Multiplier: 0.50, Tier: "Entry"}
}

// ComputeGearScoreV2 computes the v2 GearScore utilizing hardware, uptime, quality, and feeder_trust.
// GS = 0.4*Hardware + 0.3*Uptime + 0.2*Quality + 0.1*Feeder_Trust
func ComputeGearScoreV2(hwScore int, uptimeScore int, qualityScore int, feederTrust float64) GearScore {
	// Default fallbacks for unmeasured metrics
	if uptimeScore <= 0 {
		uptimeScore = 990 // Assume 99% uptime initially
	}
	if qualityScore <= 0 {
		qualityScore = 1000 // Assume 100% quality initially
	}

	rawScore := (0.4 * float64(hwScore)) + (0.3 * float64(uptimeScore)) + (0.2 * float64(qualityScore)) + (0.1 * (feederTrust * 1000.0))
	finalScore := int(math.Round(rawScore))

	if finalScore > 1000 {
		finalScore = 1000
	}
	if finalScore < 0 {
		finalScore = 0
	}

	for _, t := range gradeThresholds {
		if finalScore >= t.MinScore {
			return GearScore{
				Grade:      t.Grade,
				Score:      finalScore,
				Multiplier: t.Multiplier,
				Tier:       t.Tier,
			}
		}
	}

	return GearScore{Grade: GradeD, Score: finalScore, Multiplier: 0.50, Tier: "Entry"}
}

// ComputeRSMultiplier calculates the final RS (Reputation Score) multiplier
// using the v2 tokenomics rules described in AGENTS.md.
// RS_mult = min(GS/500, 2.0). For Anon tier, it is hard-capped at 0.5x.
func ComputeRSMultiplier(gsScore int, identityTier string) float64 {
	rsMult := float64(gsScore) / 500.0

	// General cap: max 2.0
	if rsMult > 2.0 {
		rsMult = 2.0
	}

	// Floor: never goes below 0.5. (Anon tier has base 0.5)
	if rsMult < 0.5 {
		rsMult = 0.5
	}

	// Identity Tier Cap for Anon
	if identityTier == "anon" && rsMult > 0.5 {
		rsMult = 0.5
	}

	return rsMult
}
