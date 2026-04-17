package hub

import (
	"log"
	"net"
	"sync"
	"time"
)

type TunnelManager struct {
	mu      sync.Mutex
	tunnels map[string]chan net.Conn
}

var globalTunnelManager *TunnelManager

func init() {
	globalTunnelManager = &TunnelManager{
		tunnels: make(map[string]chan net.Conn),
	}
}

func GetTunnelManager() *TunnelManager {
	return globalTunnelManager
}

// RequestTunnel prepares the manager to receive a tunnel for a specific session.
func (tm *TunnelManager) RequestTunnel(sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if _, ok := tm.tunnels[sessionID]; !ok {
		tm.tunnels[sessionID] = make(chan net.Conn, 1)
	}
}

// RegisterTunnel is called when a node connects its reverse tunnel.
func (tm *TunnelManager) RegisterTunnel(sessionID string, conn net.Conn) bool {
	tm.mu.Lock()
	ch, ok := tm.tunnels[sessionID]
	tm.mu.Unlock()

	if !ok {
		log.Printf("[Tunnel] Dropping tunnel registration for unknown session: %s", sessionID)
		return false
	}

	select {
	case ch <- conn:
		return true
	default:
		log.Printf("[Tunnel] Tunnel already registered for session: %s", sessionID)
		return false
	}
}

// AwaitTunnel waits for the node to connect its reverse tunnel.
func (tm *TunnelManager) AwaitTunnel(sessionID string, timeout time.Duration) (net.Conn, bool) {
	tm.mu.Lock()
	ch, ok := tm.tunnels[sessionID]
	tm.mu.Unlock()

	if !ok {
		return nil, false
	}

	defer func() {
		tm.mu.Lock()
		delete(tm.tunnels, sessionID)
		tm.mu.Unlock()
	}()

	select {
	case conn := <-ch:
		return conn, true
	case <-time.After(timeout):
		return nil, false
	}
}
