package models

import "testing"

func TestSwapGuardTriggersOnLargeJump(t *testing.T) {
	ok, _ := CheckAndUpdateSwapGuard(1.0)
	if !ok {
		t.Fatalf("first quote should pass")
	}
	ok, reason := CheckAndUpdateSwapGuard(1.5)
	if ok {
		t.Fatalf("expected breaker on 50%% jump")
	}
	if reason == "" {
		t.Fatalf("expected reason for breaker")
	}
}
