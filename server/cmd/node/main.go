package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// Message structure matching the server's hub/client.go
type Message struct {
	Type       string          `json:"type"`
	DeviceID   string          `json:"device_id,omitempty"`
	Secret     string          `json:"secret,omitempty"`
	DeviceType string          `json:"device_type,omitempty"`
	Arch       string          `json:"arch,omitempty"`
	RamMB      int             `json:"ram_mb,omitempty"`
	VRAMMB     int             `json:"vram_mb,omitempty"`
	HasGPU     bool            `json:"has_gpu,omitempty"`
	CPUCores   int             `json:"cpu_cores,omitempty"`
	TaskID     string          `json:"task_id,omitempty"`
	TaskType   string          `json:"task_type,omitempty"`
	InputURL   string          `json:"input_url,omitempty"`
	OutputURL  string          `json:"output_url,omitempty"`
	ResultHash string          `json:"result_hash,omitempty"`
	Bytes      int64           `json:"bytes,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type NodeCLI struct {
	DeviceID   string
	Secret     string
	ServerURL  string
	Conn       *websocket.Conn
	DeviceType string
	Arch       string
	RamMB      int
	VRAMMB     int
	CPUCores   int
}

func main() {
	_ = godotenv.Load()

	serverURL := os.Getenv("Exra_WS_URL")
	if serverURL == "" {
		serverURL = "ws://localhost:8080/ws"
	}
	secret := os.Getenv("NODE_SECRET")
	if secret == "" {
		secret = "Exra_node_default_secret"
	}

	deviceID := getOrCreateDeviceID()
	log.Printf("Starting Exra PC Node: %s", deviceID)

	node := &NodeCLI{
		DeviceID:  deviceID,
		Secret:    secret,
		ServerURL: serverURL,
	}

	node.scanHardware()
	node.Connect()

	// Wait for interrupt
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt

	log.Println("Shutting down...")
	if node.Conn != nil {
		node.Conn.Close()
	}
}

func getOrCreateDeviceID() string {
	idFile := ".Exra_id"
	data, err := os.ReadFile(idFile)
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	newID := uuid.New().String()
	_ = os.WriteFile(idFile, []byte(newID), 0644)
	return newID
}

func (n *NodeCLI) scanHardware() {
	log.Println("Scanning hardware...")

	// Host info
	h, _ := host.Info()
	n.DeviceType = fmt.Sprintf("PC (%s %s)", h.OS, h.Platform)
	n.Arch = runtime.GOARCH

	// CPU info
	c, _ := cpu.Info()
	if len(c) > 0 {
		n.Arch = fmt.Sprintf("%s (%s)", n.Arch, c[0].ModelName)
		n.CPUCores = len(c)
	}

	// RAM info
	v, _ := mem.VirtualMemory()
	n.RamMB = int(v.Total / 1024 / 1024)

	n.VRAMMB = 0
	if n.RamMB > 12288 { // Lowered for testing
		log.Println("High RAM detected, simulating GPU tier availability")
		n.VRAMMB = 4096 
	}
	n.CPUCores = 8

	log.Printf("Arch: %s, Cores: %d, RAM: %d MB, VRAM: %d MB", n.Arch, n.CPUCores, n.RamMB, n.VRAMMB)
}

func (n *NodeCLI) Connect() {
	u, err := url.Parse(n.ServerURL)
	if err != nil {
		log.Fatal("Invalid server URL:", err)
	}

	log.Printf("Connecting to %s...", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Printf("Dial failed: %v. Retrying in 5s...", err)
		time.AfterFunc(5*time.Second, n.Connect)
		return
	}
	n.Conn = c

	// Registration
	regMsg := Message{
		Type:       "register",
		DeviceID:   n.DeviceID,
		Secret:     n.Secret,
		DeviceType: n.DeviceType,
		Arch:       n.Arch,
		RamMB:      n.RamMB,
		VRAMMB:     n.VRAMMB,
		CPUCores:   n.CPUCores,
	}
	if err := n.Conn.WriteJSON(regMsg); err != nil {
		log.Printf("Register failed: %v", err)
		return
	}

	go n.readLoop()
	go n.heartbeatLoop()
}

func (n *NodeCLI) readLoop() {
	for {
		var msg Message
		err := n.Conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Read error: %v", err)
			n.Connect() // Reconnect
			return
		}

		switch msg.Type {
		case "ping":
			n.Conn.WriteJSON(Message{Type: "pong", DeviceID: n.DeviceID})
		case "compute_task":
			log.Printf("[Compute] Received task %s (%s)", msg.TaskID, msg.TaskType)
			n.handleComputeTask(msg)
		case "pop_payout":
			log.Printf("[PoP] Reward received: %s", string(msg.Payload))
		case "registered":
			log.Printf("Successfully registered on Exra Hub")
		}
	}
}

func (n *NodeCLI) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if n.Conn != nil {
			_ = n.Conn.WriteJSON(Message{Type: "heartbeat", DeviceID: n.DeviceID, CPUCores: n.CPUCores, RamMB: n.RamMB, VRAMMB: n.VRAMMB, Arch: n.Arch})
		}
	}
}

func (n *NodeCLI) handleComputeTask(t Message) {
	log.Printf("Processing compute task %s...", t.TaskID)
	
	// Simulate work
	time.Sleep(3 * time.Second)

	result := Message{
		Type:       "compute_result",
		DeviceID:   n.DeviceID,
		TaskID:     t.TaskID,
		ResultHash: fmt.Sprintf("sha256:pc_node_%s_%d", t.TaskID, time.Now().Unix()),
		OutputURL:  "https://cdn.Exra.io/results/" + t.TaskID + ".json",
	}

	if err := n.Conn.WriteJSON(result); err != nil {
		log.Printf("Failed to send result: %v", err)
	} else {
		log.Printf("Task %s completed and result sent", t.TaskID)
	}
}

