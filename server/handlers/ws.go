package handlers

import (
	"exra/hub"
	"exra/middleware"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
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
