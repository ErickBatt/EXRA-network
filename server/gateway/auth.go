package main

import "exra/gwclaims"

// VerifyToken validates the session JWT presented to the Gateway. Signing keys
// live in the Control Plane; the Gateway only holds the verifying key. The
// previous HS256 shared-secret scheme with a hardcoded default fallback has
// been removed (AUDIT_MARKETPLACE_v2.4.1 §1 D1).
func VerifyToken(tokenString string) (*gwclaims.Claims, error) {
	return gwclaims.Verify(tokenString)
}
