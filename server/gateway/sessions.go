package main

import (
	"context"
	"net"
	"sync"
	"time"
)

// pendingSessions stores the first Stitch arrival for a sessionID until the
// second party joins or the session times out.
//
// AUDIT_MARKETPLACE_v2.4.1 §1 A1/A2:
//
//   - A1 race: the old implementation let a goroutine stash its conn in the
//     counterpart channel while the partner was simultaneously leaving via
//     ctx.Done(), leaking the stashed conn and handing back a closed one.
//     Fix: sync.Once-guarded finalization makes either success or cancellation
//     win exclusively.
//
//   - A1 alternative (late second party): a Stitch caller with no prior first
//     party used to create a fresh orphan waiter and hang for 30s. Fix:
//     sessionKnownFn gates whether we even create a waiter — in production it
//     is swapped to a Redis `sessions:<sid>` existence check in billing.go.
//
//   - A2 role collision: second arrival with same role used to reject itself
//     but left the first party blocked. Fix: close the first party's done
//     channel on collision so it wakes up immediately.
var pendingSessions sync.Map // sessionID (string) -> *sessionWaiter

// firstPartyTimeout bounds how long the first arrival will wait for its
// counterpart. Production default; test_setup_test.go overrides this to a
// shorter value.
var firstPartyTimeout = 30 * time.Second

// sessionKnownFn is consulted before creating a waiter. Returning false tells
// Stitch to fast-fail (late arrival after counterpart timed out, or unknown
// session_id). Default is permissive (suitable for environments without a
// state store); billing.go swaps in a Redis HEXISTS check at gateway startup,
// and test_setup_test.go swaps in a per-test whitelist.
var sessionKnownFn = func(sessionID string) bool { return true }

type sessionWaiter struct {
	firstConn    net.Conn
	firstRole    string
	counterpart  chan net.Conn // buffered(1); populated by the second party
	done         chan struct{} // closed on any finalization (success/collision/timeout)
	finalizeOnce sync.Once
}

func (w *sessionWaiter) finalize(sessionID string) {
	w.finalizeOnce.Do(func() {
		close(w.done)
		pendingSessions.Delete(sessionID)
	})
}

// Stitch waits for the counterpart of a session.
// Returns (partnerConn, true) on successful pairing, or (nil, false) if the
// session timed out, the role collided, the session_id is unknown, or the
// counterpart failed to arrive.
func Stitch(sessionID string, role string, conn net.Conn) (net.Conn, bool) {
	if !sessionKnownFn(sessionID) {
		_ = conn.Close()
		return nil, false
	}

	w := &sessionWaiter{
		firstConn:   conn,
		firstRole:   role,
		counterpart: make(chan net.Conn, 1),
		done:        make(chan struct{}),
	}
	existing, loaded := pendingSessions.LoadOrStore(sessionID, w)
	if loaded {
		waiter := existing.(*sessionWaiter)

		// Role collision: second party with the same role as the first.
		// Finalize the first (wakes it up) and reject the second.
		if waiter.firstRole == role {
			waiter.finalize(sessionID)
			_ = conn.Close()
			return nil, false
		}

		// Happy path: hand our conn to the first party and take theirs.
		// finalizeOnce ensures that if both ctx.Done (inside the first
		// party's select) and this delivery race, exactly one wins.
		delivered := false
		waiter.finalizeOnce.Do(func() {
			waiter.counterpart <- conn
			pendingSessions.Delete(sessionID)
			close(waiter.done)
			delivered = true
		})
		if !delivered {
			_ = conn.Close()
			return nil, false
		}
		return waiter.firstConn, true
	}

	// We are the first party. Wait for counterpart, collision, or timeout.
	ctx, cancel := context.WithTimeout(context.Background(), firstPartyTimeout)
	defer cancel()

	select {
	case partner := <-w.counterpart:
		return partner, true
	case <-w.done:
		// Closed by either a role collision or an already-completed delivery
		// (we lost the finalizeOnce race).
		select {
		case partner := <-w.counterpart:
			return partner, true
		default:
			_ = conn.Close()
			return nil, false
		}
	case <-ctx.Done():
		w.finalize(sessionID)
		_ = conn.Close()
		return nil, false
	}
}
