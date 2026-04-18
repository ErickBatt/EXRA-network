package handlers

import (
	"exra/hub"
	"exra/middleware"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

// allowedOrigins is the CSRF origin whitelist for WebSocket upgrades
// (AUDIT §1 D3). Populated from WS_ALLOWED_ORIGINS (comma-separated). A
// request with no Origin header is accepted so native Android/desktop nodes
// — which do not set Origin — keep working; browser-originating requests
// must match the whitelist exactly. Empty whitelist disables the check
// (dev-only posture; production deployments MUST set this).
var allowedOrigins = func() map[string]struct{} {
	raw := strings.TrimSpace(os.Getenv("WS_ALLOWED_ORIGINS"))
	if raw == "" {
		return nil
	}
	set := make(map[string]struct{})
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			set[o] = struct{}{}
		}
	}
	return set
}()

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Native clients (Android/desktop nodes) don't send Origin.
			return true
		}
		if allowedOrigins == nil {
			// Whitelist not configured — dev mode, accept all. Log once
			// per upgrade so operators see they're running open.
			log.Printf("[WS] WARNING: WS_ALLOWED_ORIGINS unset, accepting Origin=%q", origin)
			return true
		}
		_, ok := allowedOrigins[origin]
		if !ok {
			log.Printf("[WS] Rejecting Origin=%q (CSRF guard, AUDIT §1 D3)", origin)
		}
		return ok
	},
}

func WsHandler(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if host := r.Header.Get("X-Forwarded-For"); host != "" {
			ip = strings.Split(host, ",")[0]
		}

		headerASN := strings.TrimSpace(r.Header.Get("X-ASN-Org"))
		asnOrg, isResidential := middleware.LookupASN(ip, headerASN)

		// Check residential status BEFORE upgrading
		if !isResidential {
			log.Printf("[WS] Rejecting non-residential IP: %s (ASN: %s)", ip, asnOrg)
			http.Error(w, "datacenter ip is not eligible for rewards", http.StatusForbidden)
			return
		}

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[WS] Upgrade failed for %s: %v", ip, err)
			http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
			return
		}

		client := &hub.Client{
			Hub:     h,
			Conn:    conn,
			IP:      strings.TrimSpace(ip),
			Country: strings.TrimSpace(r.URL.Query().Get("country")),
			ASNOrg:  asnOrg,
			Send:    make(chan []byte, 64),
		}

		h.Register(client)
		go client.WritePump()
		client.ReadPump()
	}
}
