package handlers

import (
	"encoding/json"
	"errors"
	"exra/hub"
	"exra/metrics"
	"exra/middleware"
	"exra/models"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
)

var ratePerGB float64 = 0.30

func SetRatePerGB(rate string) {
	r, err := strconv.ParseFloat(rate, 64)
	if err == nil {
		ratePerGB = r
	}
}

// POST /api/session/start — creates a session and returns node info
func StartSession(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	if buyer.BalanceUSD <= 0 {
		jsonError(w, "insufficient balance", http.StatusPaymentRequired)
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var (
		node *models.Node
		err  error
	)
	if req.NodeID != "" {
		node, err = models.GetActiveNodeByID(req.NodeID)
		if err != nil {
			jsonError(w, "selected node is unavailable", http.StatusServiceUnavailable)
			return
		}
	} else {
		node, err = models.GetBestNode()
		if err != nil {
			jsonError(w, "no available nodes", http.StatusServiceUnavailable)
			return
		}
	}

	session, err := models.CreateSession(buyer.ID, node.ID)
	if err != nil {
		jsonError(w, "failed to create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	metrics.TotalSessions.Inc()

	jsonResponse(w, map[string]interface{}{
		"session_id":   session.ID,
		"node_address": node.Address,
		"node_port":    node.Port,
		"node_country": node.Country,
	}, http.StatusCreated)
}

// POST /api/session/{id}/end
func EndSession(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	sessionID := mux.Vars(r)["id"]

	session, charged, err := models.FinalizeSession(sessionID, buyer.ID, 0, ratePerGB)
	if err != nil {
		if errors.Is(err, models.ErrSessionNotFound) {
			jsonError(w, "session not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, models.ErrInsufficientBuyerBalance) {
			jsonError(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		jsonError(w, "failed to end session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"session": session,
		"charged": charged,
	}, http.StatusOK)
}

// CONNECT /proxy — HTTP CONNECT tunneling proxy
// Buyers use this as an HTTP proxy endpoint. Their requests are tunneled
// through the best available node.
func HTTPConnectProxy(w http.ResponseWriter, r *http.Request) {
	buyer := middleware.BuyerFromContext(r)
	if buyer.BalanceUSD <= 0 {
		http.Error(w, "407 Payment Required", http.StatusProxyAuthRequired)
		return
	}

	country := r.Header.Get("X-Exra-Country")
	var node *models.Node
	var err error
	if country != "" {
		node, err = models.GetActiveNodeByCountry(country)
		if err != nil {
			log.Printf("No active node for country=%s, using fallback: %v", country, err)
		}
	}
	if node == nil {
		node, err = models.GetBestNode()
		if err != nil {
			http.Error(w, "503 No nodes available", http.StatusServiceUnavailable)
			return
		}
	}

	session, err := models.CreateSession(buyer.ID, node.ID)
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}

	metrics.TotalSessions.Inc()
	log.Printf("Proxy session created session_id=%s buyer_id=%s node_id=%s country=%s", session.ID, buyer.ID, node.ID, country)
	if runtimeHub != nil && node.DeviceID != "" {
		// Signal node to open a reverse tunnel
		targetPort := 80
		if r.URL.Port() != "" {
			targetPort, _ = strconv.Atoi(r.URL.Port())
		} else if r.Method == http.MethodConnect {
			targetPort = 443
			if hostParts := strings.Split(r.Host, ":"); len(hostParts) > 1 {
				targetPort, _ = strconv.Atoi(hostParts[1])
			}
		}

		targetHost := r.Host
		if hostParts := strings.Split(r.Host, ":"); len(hostParts) > 1 {
			targetHost = hostParts[0]
		}

		hub.GetTunnelManager().RequestTunnel(session.ID)
		runtimeHub.BroadcastProxyOpen(node.DeviceID, session.ID, targetHost, targetPort)
	}

	// Ensure session is always finalized even if the tunnel never connects.
	sessionFinalized := false
	finalizeOnce := func() {
		if !sessionFinalized {
			sessionFinalized = true
			models.FinalizeSession(session.ID, buyer.ID, 0, ratePerGB) //nolint:errcheck
		}
	}
	defer func() {
		if !sessionFinalized {
			finalizeOnce()
		}
	}()

	// Wait for node to connect the reverse tunnel
	nodeConn, ok := hub.GetTunnelManager().AwaitTunnel(session.ID, 15*time.Second)
	if !ok {
		log.Printf("Timeout waiting for node %s to open tunnel for session %s, trying failover", node.ID, session.ID)
		failoverNode, ferr := models.GetBestNode()
		if ferr != nil || failoverNode == nil || failoverNode.ID == node.ID {
			http.Error(w, "504 Gateway Timeout", http.StatusGatewayTimeout)
			return
		}
		if runtimeHub != nil && failoverNode.DeviceID != "" {
			targetPort := 80
			if r.URL.Port() != "" {
				targetPort, _ = strconv.Atoi(r.URL.Port())
			} else if r.Method == http.MethodConnect {
				targetPort = 443
				if hostParts := strings.Split(r.Host, ":"); len(hostParts) > 1 {
					targetPort, _ = strconv.Atoi(hostParts[1])
				}
			}
			targetHost := r.Host
			if hostParts := strings.Split(r.Host, ":"); len(hostParts) > 1 {
				targetHost = hostParts[0]
			}
			hub.GetTunnelManager().RequestTunnel(session.ID)
			runtimeHub.BroadcastProxyOpen(failoverNode.DeviceID, session.ID, targetHost, targetPort)
		}
		nodeConn, ok = hub.GetTunnelManager().AwaitTunnel(session.ID, 10*time.Second)
		if !ok {
			http.Error(w, "504 Gateway Timeout", http.StatusGatewayTimeout)
			return
		}
	}
	defer nodeConn.Close()

	// Set baseline deadlines to prevent indefinitely hung connections (REC-8)
	nodeConn.SetDeadline(time.Now().Add(5 * time.Minute))

	// For CONNECT method, buyer connection is already a raw TCP stream after hijacking.
	// Non-CONNECT requests need to be forwarded as raw HTTP request bytes first.
	if r.Method != http.MethodConnect {
		r.WriteProxy(nodeConn)
	}

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "500 Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "500 Hijack failed", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	if r.Method == http.MethodConnect {
		clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	}

	// Bidirectional copy with byte counting. Wait for both directions to exit.
	var totalBytes int64
	done := make(chan struct{}, 2)
	copyErr := make(chan error, 2)

	closeWrite := func(c net.Conn) {
		if tcp, ok := c.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}

	go func() {
		n, err := io.Copy(nodeConn, clientConn)
		atomic.AddInt64(&totalBytes, n)
		closeWrite(nodeConn)
		copyErr <- err
		done <- struct{}{}
	}()
	go func() {
		n, err := io.Copy(clientConn, nodeConn)
		atomic.AddInt64(&totalBytes, n)
		closeWrite(clientConn)
		copyErr <- err
		done <- struct{}{}
	}()

	<-done
	<-done
	err1 := <-copyErr
	err2 := <-copyErr
	if err1 != nil && !errors.Is(err1, net.ErrClosed) {
		log.Printf("proxy copy client->node error: %v", err1)
	}
	if err2 != nil && !errors.Is(err2, net.ErrClosed) {
		log.Printf("proxy copy node->client error: %v", err2)
	}

	// Finalize session and charge exactly once.
	bytesTransferred := atomic.LoadInt64(&totalBytes)
	if bytesTransferred > 0 {
		metrics.TotalBytesProxied.Add(float64(bytesTransferred))
	}

	sessionFinalized = true
	finalized, charged, err := models.FinalizeSession(session.ID, buyer.ID, bytesTransferred, ratePerGB)
	if err != nil {
		log.Printf("Failed to finalize session %s: %v", session.ID, err)
		return
	}
	log.Printf(
		"Session %s ended: %d bytes, $%.6f, charged=%t",
		finalized.ID, finalized.BytesUsed, finalized.CostUSD, charged,
	)
}

// GET /api/node/tunnel?session_id=...
// Nodes connect here to provide raw TCP tunnel for a specific session.
//
// AUDIT §1 G1: previously this handler accepted any nodeAuth-authenticated
// caller claiming any session_id, so any registered node could steal an
// in-flight session's data plane. We now require the caller to prove they
// are the node the matcher bound to the session:
//
//  1. Session must exist in Redis and have a recorded node_did.
//  2. Caller must supply X-Device-ID + X-Device-Sig (hex sr25519 sig over
//     the raw session_id).
//  3. The claimed device_id must resolve to the same DID the matcher stored
//     against the session.
//  4. The signature must verify under that node's registered public key.
//
// If any step fails we return 403 before the hijack — the bogus caller is
// not granted a raw TCP tunnel.
func TunnelHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	claimedDeviceID := r.Header.Get("X-Device-ID")
	claimedSig := r.Header.Get("X-Device-Sig")
	if claimedDeviceID == "" || claimedSig == "" {
		http.Error(w, "forbidden: missing device proof", http.StatusForbidden)
		return
	}

	if runtimeHub == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	expectedDID, err := runtimeHub.GetSessionNodeDID(r.Context(), sessionID)
	if err != nil {
		log.Printf("[Tunnel] G1 session lookup failed session=%s err=%v", sessionID, err)
		http.Error(w, "session lookup failed", http.StatusInternalServerError)
		return
	}
	if expectedDID == "" {
		http.Error(w, "forbidden: unknown session", http.StatusForbidden)
		return
	}

	pubKey, did, err := models.GetNodeAuthByDeviceID(claimedDeviceID)
	if err != nil || pubKey == "" || did == "" {
		log.Printf("[Tunnel] G1 rejecting device=%s: node record missing (err=%v)", claimedDeviceID, err)
		http.Error(w, "forbidden: unknown device", http.StatusForbidden)
		return
	}
	if did != expectedDID {
		log.Printf("[Tunnel] G1 rejecting device=%s: DID mismatch (claimed=%s expected=%s) session=%s", claimedDeviceID, did, expectedDID, sessionID)
		http.Error(w, "forbidden: node not authorised for session", http.StatusForbidden)
		return
	}
	okSig, err := middleware.VerifyDIDSignature(pubKey, sessionID, claimedSig)
	if err != nil || !okSig {
		log.Printf("[Tunnel] G1 rejecting device=%s: invalid signature (err=%v)", claimedDeviceID, err)
		http.Error(w, "forbidden: invalid signature", http.StatusForbidden)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		return
	}

	if !hub.GetTunnelManager().RegisterTunnel(sessionID, conn) {
		conn.Close()
	}
}
