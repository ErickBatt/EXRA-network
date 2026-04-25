package hub

import (
	"encoding/json"
	"exra/db"
	"exra/metrics"
	"exra/middleware"
	"exra/models"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type Message struct {
	Type             string  `json:"type"`
	DeviceID         string  `json:"device_id,omitempty"`
	Country          string  `json:"country,omitempty"`
	DeviceType       string  `json:"device_type,omitempty"`
	Bytes            int64   `json:"bytes,omitempty"`
	SessionID        string  `json:"session_id,omitempty"`
	ASNOrg           string  `json:"asn_org,omitempty"`
	ReferrerDeviceID string  `json:"referrer_device_id,omitempty"`
	Secret           string  `json:"secret,omitempty"`
	PricePerGB       float64 `json:"price_per_gb,omitempty"`
	AutoPrice        *bool   `json:"auto_price,omitempty"`

	Arch     string `json:"arch,omitempty"`
	RamMB    int    `json:"ram_mb,omitempty"`
	VRAMMB   int    `json:"vram_mb,omitempty"`
	HasGPU   bool   `json:"has_gpu,omitempty"`
	CPUCores int    `json:"cpu_cores,omitempty"`

	PubKey    string `json:"pub_key,omitempty"`
	DID       string `json:"did,omitempty"`
	Signature string `json:"signature,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`

	// Compute Tasks
	TaskID     string `json:"task_id,omitempty"`
	TaskType   string `json:"task_type,omitempty"`
	InputURL   string `json:"input_url,omitempty"`
	OutputURL  string `json:"output_url,omitempty"`
	ResultHash string `json:"result_hash,omitempty"`

	// Link verification
	Approved  *bool  `json:"approved,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	HWHash    string `json:"hw_hash,omitempty"` // hardware fingerprint sent by Android on approval (#6)

	// Hardware requirements for dispatcher
	MinVRAMMB   int `json:"min_vram_mb,omitempty"`
	MinCPUCores int `json:"min_cpu_cores,omitempty"`

	// Feeder Audit (Anti-Fraud)
	AssignmentID int64  `json:"assignment_id,omitempty"`
	Verdict      string `json:"verdict,omitempty"`
	TargetIP     string `json:"target_ip,omitempty"`
	TargetPort   int    `json:"target_port,omitempty"`

	// Nested Data field for consistency with Android ExraWsClient
	Data struct {
		DID         string `json:"did,omitempty"`
		Timestamp   int64  `json:"timestamp,omitempty"`
		Signature   string `json:"signature,omitempty"`
		TaskID      string `json:"task_id,omitempty"`
		Result      any    `json:"result,omitempty"`
		Attestation struct {
			ResultHash string `json:"result_hash,omitempty"`
			Timestamp  int64  `json:"timestamp,omitempty"`
			DID        string `json:"did,omitempty"`
			Signature  string `json:"signature,omitempty"`
		} `json:"attestation,omitempty"`
	} `json:"data,omitempty"`
}

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	DeviceID string
	IP       string
	Country  string
	ASNOrg   string
	PubKey   string
	DID      string
	Lat      float64
	Lng      float64
	Send     chan []byte

	// Rate limiting
	msgCount    int
	windowStart time.Time

	// AUDIT §1 C3: close(c.Send) used to be called from the Hub goroutine
	// while ReadPump / Unregister paths could also trigger a close on the
	// same channel, producing a "close of closed channel" panic that took
	// the whole hub down. closeOnce serializes close; sendClosed lets
	// senders check before writing so they skip the channel instead of
	// panicking on "send on closed channel" during shutdown races.
	closeOnce   sync.Once
	sendClosed  atomic.Bool
}

// CloseSend closes c.Send exactly once. Safe to call from any goroutine.
// After CloseSend returns, TrySend will return false instead of writing.
func (c *Client) CloseSend() {
	c.closeOnce.Do(func() {
		c.sendClosed.Store(true)
		close(c.Send)
	})
}

// verifyPopSignature validates the DID signature on a heartbeat/pong message.
// Returns false (and logs the rejection reason) when:
//   - signature or timestamp is missing
//   - timestamp is stale (>5 min old) — replay protection
//   - DID signature is invalid
//
// This is the mandatory gate for all PoP reward grants (E4 + pong-bypass fix).
func verifyPopSignature(deviceID, pubKey string, timestamp int64, sig string) bool {
	if sig == "" || timestamp == 0 {
		log.Printf("[Security] PoP rejected device=%s: missing signature or timestamp", deviceID)
		return false
	}
	ts := time.Unix(timestamp, 0)
	if age := time.Since(ts); age > 5*time.Minute || age < -time.Minute {
		log.Printf("[Security] PoP rejected device=%s: stale timestamp age=%v", deviceID, age)
		return false
	}
	ok, err := middleware.VerifyDIDSignature(pubKey, fmt.Sprintf("%d", timestamp), sig)
	if err != nil || !ok {
		log.Printf("[Security] PoP rejected device=%s: invalid DID signature", deviceID)
		return false
	}
	return true
}

// TrySend queues a message on c.Send without blocking. It returns false
// if the channel is closed or full. Callers MUST use this instead of
// `c.Send <- msg` anywhere a shutdown race is possible (AUDIT §1 C3).
func (c *Client) TrySend(msg []byte) bool {
	if c.sendClosed.Load() {
		return false
	}
	select {
	case c.Send <- msg:
		return true
	default:
		return false
	}
}

const (
	rateLimitWindow  = 10 * time.Second
	rateLimitMaxMsgs = 20
)

func (c *Client) ReadPump() {
	defer func() {
		if c.DeviceID != "" {
			if err := models.SetNodeOfflineByDeviceID(c.DeviceID); err != nil {
				log.Printf("Failed to set node offline for %s: %v", c.DeviceID, err)
			}
		}
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	// WS-level pong is a keepalive control frame only — no PoP reward.
	// PoP is earned exclusively via signed "heartbeat" or "pong" JSON messages
	// so that rewards require a verifiable DID signature (E4 / pong-bypass fix).
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		if c.DeviceID != "" {
			_, _ = models.UpsertWSNode(c.DeviceID, c.PubKey, c.IP, c.Country, "unknown", "", c.ASNOrg, true, "", 0, 0, 0, true, 0, c.DID)
		}
		return nil
	})

	for {
		_, payload, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}

		// Rate limiting check
		now := time.Now()
		if now.Sub(c.windowStart) > rateLimitWindow {
			c.windowStart = now
			c.msgCount = 0
		}
		c.msgCount++
		if c.msgCount > rateLimitMaxMsgs {
			log.Printf("[Security] Rate limit exceeded for node IP=%s device=%s. Closing.", c.IP, c.DeviceID)
			return // Exiting ReadPump closes connection
		}

		metrics.HubMessagesIn.Inc()

		var msg Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "register":
			if msg.DeviceID == "" || msg.PubKey == "" || msg.Signature == "" || msg.DID == "" {
				log.Printf("[Security] Rejecting node registration device_id=%s: missing credentials (pubkey/did/sig)", msg.DeviceID)
				return
			}

			// Verify PEAQ DID Signature (Sign("$deviceId:$did:$timestamp"))
			signMsg := msg.DeviceID + ":" + msg.DID + ":" + msg.Timestamp
			if msg.Timestamp == "" {
				signMsg = msg.DeviceID + ":" + msg.DID
			}

			ok, err := middleware.VerifyDIDSignature(msg.PubKey, signMsg, msg.Signature)
			if err != nil || !ok {
				log.Printf("[Security] Rejecting node registration device_id=%s: invalid signature (err=%v)", msg.DeviceID, err)
				resp, _ := json.Marshal(map[string]string{
					"type":    "error",
					"message": "authentication failed: invalid peaq DID signature",
				})
				c.Conn.WriteMessage(websocket.TextMessage, resp)
				return
			}

			c.Hub.BindClientDeviceID(c, msg.DeviceID)
			c.PubKey = msg.PubKey
			c.DID = msg.DID
			if msg.Country != "" {
				c.Country = msg.Country
			}

			tier := "network"
			if (strings.Contains(msg.Arch, "amd64") || strings.Contains(msg.Arch, "x86") || msg.HasGPU || msg.VRAMMB > 2000) && msg.RamMB >= 8192 {
				tier = "compute"
			}

			autoPrice := true
			if msg.AutoPrice != nil {
				autoPrice = *msg.AutoPrice
			}
			node, err := models.UpsertWSNode(c.DeviceID, c.PubKey, c.IP, c.Country, msg.DeviceType, tier, c.ASNOrg, true, msg.Arch, msg.CPUCores, msg.VRAMMB, msg.RamMB, autoPrice, msg.PricePerGB, c.DID)
			if err != nil {
				log.Printf("Node register upsert failed: %v", err)
				continue
			}
			// Bind referrer on first registration if provided.
			if msg.ReferrerDeviceID != "" {
				if err := models.SetReferrer(c.DeviceID, msg.ReferrerDeviceID); err != nil {
					log.Printf("SetReferrer failed device_id=%s referrer=%s err=%v", c.DeviceID, msg.ReferrerDeviceID, err)
				}
			}
			resp, _ := json.Marshal(map[string]string{
				"type":    "registered",
				"node_id": node.ID,
				"message": "Welcome to Exra",
			})
			c.Send <- resp
			log.Printf("WS node registered device_id=%s country=%s", c.DeviceID, c.Country)

			// ── Hardening: Send Timelock Stats for Anon nodes ──
			if node.IdentityTier == "anon" {
				unlockTs := node.CreatedAt.Add(time.Duration(24) * time.Hour).Unix()
				statusResp, _ := json.Marshal(map[string]interface{}{
					"type":             "timelock_update",
					"identity_tier":    "anon",
					"unlock_timestamp": unlockTs,
					"rs_mult":          node.RSScore / 500.0,
				})
				c.Send <- statusResp
			}
		case "heartbeat":
			// Signature is now MANDATORY — no signature = no PoP (E4 fix).
			// The previous optional guard has been removed: a client that omits
			// the field must be treated the same as one with a bad signature.
			if c.DeviceID != "" {
				if !verifyPopSignature(c.DeviceID, c.PubKey, msg.Data.Timestamp, msg.Data.Signature) {
					continue
				}
				_, _ = models.UpsertWSNode(c.DeviceID, c.PubKey, c.IP, c.Country, "unknown", "", c.ASNOrg, true, msg.Arch, msg.CPUCores, msg.VRAMMB, msg.RamMB, true, msg.PricePerGB, c.DID)
				if err := models.HeartbeatPoP(c.DeviceID, models.GetPopEmission()); err != nil {
					log.Printf("[PoP] heartbeat msg failed device_id=%s err=%v", c.DeviceID, err)
				}
			}
		case "pong":
			// App-level pong also requires a signed timestamp (pong-bypass fix).
			// Without this gate any connected node could spam "pong" for free PoP.
			if c.DeviceID != "" {
				if !verifyPopSignature(c.DeviceID, c.PubKey, msg.Data.Timestamp, msg.Data.Signature) {
					continue
				}
				_, _ = models.UpsertWSNode(c.DeviceID, c.PubKey, c.IP, c.Country, "unknown", "", c.ASNOrg, true, msg.Arch, msg.CPUCores, msg.VRAMMB, msg.RamMB, true, msg.PricePerGB, c.DID)
				if err := models.HeartbeatPoP(c.DeviceID, models.GetPopEmission()); err != nil {
					log.Printf("[PoP] pong msg failed device_id=%s err=%v", c.DeviceID, err)
				}
			}
		case "traffic":
			// AUDIT §1 E3: worker-reported byte counters used to be trusted
			// unconditionally — a hostile node could multiply msg.Bytes by
			// 10 and inflate rewards. We clamp each report against
			// MaxTrafficPerSec (realistic ceiling between ping intervals,
			// 1 GiB) so a single report can't inflate by an order of
			// magnitude. The proper buyer_reported cross-check still needs
			// to land in models/session.go::FinalizeSession comparing
			// against the Gateway's settled byte count; this clamp is the
			// minimum viable defence.
			const MaxTrafficPerSec int64 = 1 << 30 // 1 GiB per report ceiling
			if c.DeviceID != "" && msg.Bytes > 0 {
				if msg.Bytes > MaxTrafficPerSec {
					log.Printf("[Security] traffic report from %s exceeds MaxTrafficPerSec=%d bytes=%d — clamping", c.DeviceID, MaxTrafficPerSec, msg.Bytes)
					msg.Bytes = MaxTrafficPerSec
				}
				if err := models.AddNodeTrafficByDeviceID(c.DeviceID, msg.Bytes); err != nil {
					log.Printf("Traffic update failed device_id=%s err=%v", c.DeviceID, err)
				}
			}
		case "proxy_result":
			var res ProxyResult
			if err := json.Unmarshal(payload, &res); err != nil {
				log.Printf("Failed to parse proxy_result: %v", err)
				continue
			}
			if res.SessionID != "" {
				c.Hub.BroadcastProxyResult(&res)
				log.Printf("Proxy result received session_id=%s status=%d bytes=%d err=%s", res.SessionID, res.StatusCode, res.Bytes, res.Error)
			}
		case "task_result":
			if msg.TaskID == "" {
				continue
			}
			log.Printf("[Compute] Task result received task_id=%s node_id=%s", msg.TaskID, c.DeviceID)
			err := models.CompleteTask(msg.TaskID, c.DeviceID, msg.ResultHash, msg.OutputURL)
			if err != nil {
				log.Printf("[Compute] Failed to complete task %s: %v", msg.TaskID, err)
			}
		case "link_response":
			if c.DeviceID != "" && msg.RequestID != "" && msg.Approved != nil {
				log.Printf("[TMA] Link response received device_id=%s approved=%v req_id=%s hw_hash=%q",
					c.DeviceID, *msg.Approved, msg.RequestID, msg.HWHash)
				if *msg.Approved {
					// #6: verify hardware fingerprint when Android provides one.
					if msg.HWHash != "" {
						var stored string
						_ = db.DB.QueryRow(
							`SELECT COALESCE(hw_fingerprint,'') FROM nodes WHERE device_id=$1`, c.DeviceID,
						).Scan(&stored)
						if stored != "" && stored != msg.HWHash {
							// Fingerprint mismatch — device identity changed; reject the link.
							log.Printf("[TMA] Fingerprint mismatch device_id=%s — rejecting link req_id=%s",
								c.DeviceID, msg.RequestID)
							if err := models.RejectTmaLink(msg.RequestID, c.DeviceID); err != nil {
								log.Printf("[TMA] Failed to reject mismatched link: %v", err)
							}
							break
						}
						// First-time fingerprint registration — store it.
						if stored == "" {
							if _, err := db.DB.Exec(
								`UPDATE nodes SET hw_fingerprint=$1 WHERE device_id=$2`, msg.HWHash, c.DeviceID,
							); err != nil {
								log.Printf("[TMA] Failed to store hw_fingerprint device_id=%s: %v", c.DeviceID, err)
							}
						}
					}
					if err := models.CompleteTmaLink(msg.RequestID, c.DeviceID); err != nil {
						log.Printf("[TMA] Failed to complete link: %v", err)
					}
				} else {
					if err := models.RejectTmaLink(msg.RequestID, c.DeviceID); err != nil {
						log.Printf("[TMA] Failed to reject link: %v", err)
					}
				}
			}
		case "compute_result":
			// Support both flat (legacy) and nested (new Android) structures
			taskID := msg.TaskID
			resultHash := msg.ResultHash
			if taskID == "" && msg.Data.TaskID != "" {
				taskID = msg.Data.TaskID
				resultHash = msg.Data.Attestation.ResultHash
			}

			if c.DeviceID != "" && taskID != "" {
				if err := models.CompleteTask(taskID, c.DeviceID, resultHash, msg.OutputURL); err != nil {
					log.Printf("Failed to complete task %s from node %s: %v", taskID, c.DeviceID, err)
				} else {
					metrics.ComputeTasksCompleted.Inc()
					log.Printf("Compute task completed task_id=%s node_id=%s", taskID, c.DeviceID)
				}
			}
		case "feeder_report":
			// AUDIT §1 E1: previously any registered worker could submit a
			// verdict="fail" on any neighbour and trigger that neighbour's
			// slashing pipeline. We now require a DID signature over
			// "assignmentID:target:verdict" from c.PubKey (the feeder's own
			// registered key) before the report is accepted.
			if c.DeviceID == "" || msg.AssignmentID <= 0 || msg.Verdict == "" {
				continue
			}
			if c.PubKey == "" || msg.Data.Signature == "" {
				log.Printf("[Feeder] Rejecting unsigned feeder_report from %s assignment=%d", c.DeviceID, msg.AssignmentID)
				continue
			}
			signMsg := fmt.Sprintf("%d:%s:%s", msg.AssignmentID, msg.DeviceID, msg.Verdict)
			ok, err := middleware.VerifyDIDSignature(c.PubKey, signMsg, msg.Data.Signature)
			if err != nil || !ok {
				log.Printf("[Security] Rejecting feeder_report from %s: invalid signature (err=%v)", c.DeviceID, err)
				continue
			}
			log.Printf("[Feeder] Report from %s for assignment %d: %s", c.DeviceID, msg.AssignmentID, msg.Verdict)
			if err := models.RecordFeederReport(msg.AssignmentID, c.DeviceID, msg.DeviceID, msg.Verdict, 0, 0); err != nil {
				log.Printf("[Feeder] Failed to record report for %d: %v", msg.AssignmentID, err)
			}
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
			metrics.HubMessagesOut.Inc()
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
