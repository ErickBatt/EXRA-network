package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	nodeCount   = flag.Int("nodes", 100, "number of nodes to simulate")
	serverURL   = flag.String("url", "ws://localhost:8080/ws", "websocket server URL")
	hbInterval  = flag.Int("hb", 30, "heartbeat interval in seconds")
	trafficProb = flag.Float64("traffic", 0.1, "probability of reporting traffic in each tick")
	spawnRate   = flag.Int("rate", 100, "nodes to spawn per second")
)

type HubMessage struct {
	Type      string `json:"type"`
	DeviceID  string `json:"device_id,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	Country   string `json:"country,omitempty"`
	PubKey    string `json:"pub_key,omitempty"`
	Signature string `json:"signature,omitempty"`
}

func signDeviceID(privKey *ecdsa.PrivateKey, deviceID string) (string, error) {
	hash := sha256.Sum256([]byte(deviceID))
	r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return "", err
	}
	sig := append(r.Bytes(), s.Bytes()...)
	if len(sig) < 64 {
		// Pad R and S if they are shorter than 32 bytes
		padded := make([]byte, 64)
		copy(padded[32-len(r.Bytes()):32], r.Bytes())
		copy(padded[64-len(s.Bytes()):64], s.Bytes())
		sig = padded
	}
	return hex.EncodeToString(sig), nil
}

func encodePubKey(pubKey *ecdsa.PublicKey) string {
	b, _ := x509.MarshalPKIXPublicKey(pubKey)
	return hex.EncodeToString(b)
}

func randomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func simulateNode(id int, wg *sync.WaitGroup, stop chan struct{}, activeNodes *int64) {
	defer wg.Done()

	// 1. Generate PEAQ DID Identity
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pubKeyHex := encodePubKey(&privKey.PublicKey)
	deviceID := "sim-" + randomID()
	sigHex, _ := signDeviceID(privKey, deviceID)

	u, err := url.Parse(*serverURL)
	if err != nil {
		log.Printf("[Node %d] invalid url: %v", id, err)
		return
	}
	
	q := u.Query()
	q.Set("country", "US")
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		// Log failures but don't stop the whole test
		if id%100 == 0 {
			log.Printf("[Node %d] Dial failed: %v", id, err)
		}
		return
	}
	defer conn.Close()

	atomic.AddInt64(activeNodes, 1)
	defer atomic.AddInt64(activeNodes, -1)

	// 2. Register with peaq DID Signature
	reg, _ := json.Marshal(HubMessage{
		Type:      "register",
		DeviceID:  deviceID,
		Country:   "US",
		PubKey:    pubKeyHex,
		Signature: sigHex,
	})
	if err := conn.WriteMessage(websocket.TextMessage, reg); err != nil {
		return
	}

	ticker := time.NewTicker(time.Duration(*hbInterval) * time.Second)
	defer ticker.Stop()

	// Reading pump (must consume messages to keep connection alive)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			// Heartbeat
			hb, _ := json.Marshal(HubMessage{Type: "heartbeat", DeviceID: deviceID})
			if err := conn.WriteMessage(websocket.TextMessage, hb); err != nil {
				return
			}
			
			// Optional Traffic
			if *trafficProb > (float64(id%100) / 100.0) {
				tr, _ := json.Marshal(HubMessage{
					Type:     "traffic",
					DeviceID: deviceID,
					Bytes:    int64(1024 * 1024 * 5), // 5MB fixed per tick
				})
				_ = conn.WriteMessage(websocket.TextMessage, tr)
			}
		case <-stop:
			return
		}
	}
}

func main() {
	flag.Parse()
	log.Printf("Starting High-Scale Load Test...")
	log.Printf("Target: %d nodes | Rate: %d spawn/sec | Endpoint: %s", *nodeCount, *spawnRate, *serverURL)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	var activeNodes int64

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Spawn nodes in batches
	batchSize := *spawnRate
	if batchSize <= 0 { batchSize = 1 }
	
	for i := 0; i < *nodeCount; i++ {
		wg.Add(1)
		go simulateNode(i, &wg, stop, &activeNodes)
		
		if i > 0 && i%batchSize == 0 {
			time.Sleep(1 * time.Second)
			log.Printf("Progress: %d nodes spawned (Active: %d)", i, atomic.LoadInt64(&activeNodes))
		}
	}

	log.Printf("Spawn phase complete. All %d nodes are working.", *nodeCount)

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			log.Printf("STATUS: %d nodes connected", atomic.LoadInt64(&activeNodes))
		}
	}()

	<-sigChan
	log.Println("Stopping nodes...")
	close(stop)
	wg.Wait()
	log.Println("Load test finished.")
}

