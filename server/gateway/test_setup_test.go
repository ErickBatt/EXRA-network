package main

import (
	"strings"
	"time"
)

// This file is _test.go-only and is not compiled into the production gateway
// binary. It wires the time constants and sessionKnownFn for the audit
// proof tests:
//
//   * readTimeout is shortened so TestBridge_NoReadDeadlineHangsForever can
//     complete within its 2 s bound (A3 liveness).
//   * firstPartyTimeout mirrors readTimeout for Stitch-level waits so the
//     A1/A2 tests don't wait the full 30 s default.
//   * sessionKnownFn is replaced with a substring whitelist. Test sessionIDs
//     contain the sub-test name, so we accept sessionIDs whose tests are
//     designed to exercise pairing ("role-collision") and reject the ones
//     meant to prove the fast-fail path ("orphan").
func init() {
	readTimeout = 500 * time.Millisecond
	firstPartyTimeout = 1500 * time.Millisecond
	sessionKnownFn = func(sid string) bool {
		// LateSecondParty test expects the fast-fail path; everything else
		// is a legitimate paired session.
		if strings.Contains(sid, "orphan") {
			return false
		}
		return true
	}
}
