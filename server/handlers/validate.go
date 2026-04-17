package handlers

import (
	"fmt"
	"math"
	"strings"
)

const (
	maxDeviceIDLen   = 128
	maxCountryLen    = 8
	maxWalletLen     = 256
	maxEmailLen      = 254 // RFC 5321
	maxASNOrgLen     = 256
	maxTaskTypeLen   = 64
	maxURLLen        = 2048
)

// validateDeviceID checks device_id length and characters.
func validateDeviceID(id string) error {
	if id == "" {
		return fmt.Errorf("device_id is required")
	}
	if len(id) > maxDeviceIDLen {
		return fmt.Errorf("device_id exceeds maximum length of %d", maxDeviceIDLen)
	}
	return nil
}

// validateEmail performs basic email sanity checks.
func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if len(email) > maxEmailLen {
		return fmt.Errorf("email exceeds maximum length")
	}
	if !strings.Contains(email, "@") {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// validateWallet checks recipient wallet address length.
func validateWallet(wallet string) error {
	if wallet == "" {
		return fmt.Errorf("recipient_wallet is required")
	}
	if len(wallet) > maxWalletLen {
		return fmt.Errorf("recipient_wallet exceeds maximum length")
	}
	return nil
}

// validateFloat rejects NaN, Infinity, and values outside [min, max].
func validateFloat(name string, v, min, max float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number", name)
	}
	if v < min || v > max {
		return fmt.Errorf("%s must be between %v and %v", name, min, max)
	}
	return nil
}
