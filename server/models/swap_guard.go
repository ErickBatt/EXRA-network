package models

import (
	"math"
	"sync"
	"time"
)

var (
	swapGuardMu   sync.Mutex
	lastSwapPrice float64
	lastSwapAt    time.Time
	swapPausedTo  time.Time
)

func SwapGuardState() (paused bool, until time.Time) {
	swapGuardMu.Lock()
	defer swapGuardMu.Unlock()
	return time.Now().Before(swapPausedTo), swapPausedTo
}

func CheckAndUpdateSwapGuard(price float64) (allowed bool, reason string) {
	swapGuardMu.Lock()
	defer swapGuardMu.Unlock()
	now := time.Now()
	if now.Before(swapPausedTo) {
		return false, "swap circuit breaker active"
	}
	if lastSwapPrice > 0 {
		// 30% jump guard; pause for 15 seconds.
		if math.Abs(price-lastSwapPrice)/lastSwapPrice > 0.30 {
			swapPausedTo = now.Add(15 * time.Second)
			return false, "swap paused due to abnormal volatility"
		}
	}
	lastSwapPrice = price
	lastSwapAt = now
	_ = lastSwapAt
	return true, ""
}
