package main

import (
	"context"
	"net"
	"sync"
	"time"
)

var (
	// pendingSessions stores sessions waiting for the second party to connect.
	// Key: sessionID (string), Value: *sessionWaiter
	pendingSessions sync.Map
)

type sessionWaiter struct {
	conn       net.Conn
	role       string
	connected  chan net.Conn
	cancelFunc context.CancelFunc
}

// Stitch waits for the counterpart of a session.
// If this is the first party connecting, it registers itself and waits.
// If this is the second party, it triggers the bridge and returns the counterpart connection.
func Stitch(sessionID string, role string, conn net.Conn) (net.Conn, bool) {
	val, loaded := pendingSessions.LoadOrStore(sessionID, &sessionWaiter{
		conn: conn,
		role: role,
		connected: make(chan net.Conn, 1),
	})

	waiter := val.(*sessionWaiter)

	if loaded {
		// We are the second party
		if waiter.role == role {
			// Collision: Two parties with same role for same session. Reject.
			conn.Close()
			return nil, false
		}

		// Success! Notify the first party and return their connection.
		select {
		case waiter.connected <- conn:
			pendingSessions.Delete(sessionID)
			return waiter.conn, true
		default:
			// Waiter was already fulfilled or closed
			conn.Close()
			return nil, false
		}
	}

	// We are the first party. Setup TTL timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	waiter.cancelFunc = cancel

	defer cancel()

	select {
	case counterpart := <-waiter.connected:
		return counterpart, true
	case <-ctx.Done():
		// Timeout reached. Cleanup.
		pendingSessions.Delete(sessionID)
		conn.Close()
		return nil, false
	}
}
