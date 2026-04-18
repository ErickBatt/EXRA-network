package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"exra/gwclaims"

	"github.com/gobwas/ws"
)

func main() {
	// Fail fast if no verifying key is configured. There is no hardcoded
	// fallback anymore (AUDIT §1 D1).
	gwclaims.MustInitVerifier()

	// Wire up Redis-backed billing settlement for AUDIT §1 G3.
	initBilling()

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8082"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/gateway", func(w http.ResponseWriter, r *http.Request) {
		// 1. Authenticate using Stateless JWT
		tokenString := r.URL.Query().Get("jwt")
		if tokenString == "" {
			http.Error(w, "missing jwt", http.StatusUnauthorized)
			return
		}

		claims, err := VerifyToken(tokenString)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// 2. Upgrade to WebSocket using gobwas/ws
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			log.Printf("[Gateway] Upgrade error: %v", err)
			return
		}

		log.Printf("[Gateway] Connection established: session=%s role=%s", claims.SessionID, claims.Role)

		// 3. Stitch connections
		// This block is synchronous for the first party (waits for second)
		counterpart, ok := Stitch(claims.SessionID, claims.Role, conn)
		if !ok {
			// Stitching failed or timed out (already handled inside Stitch)
			return
		}

		log.Printf("[Gateway] Bridge starting: session=%s", claims.SessionID)

		// 4. Start the Bridge. Returns total bytes shipped in each direction
		// so we can settle Redis credits on close (AUDIT §1 G3).
		b1, b2 := Bridge(conn, counterpart)
		settleSession(claims.SessionID, b1+b2)

		log.Printf("[Gateway] Bridge closed: session=%s bytes=%d", claims.SessionID, b1+b2)
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("[Gateway] High-Performance Data Plane starting on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Gateway] Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("[Gateway] Shutting down...")
	server.Close()
}
