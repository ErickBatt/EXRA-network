package hub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"exra/metrics"
	"exra/models"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis Keys for Marketplace v2.1
const (
	KeyPrefixNodes    = "nodes:%s:%s" // nodes:country:rs_tier
	KeyPrefixSessions = "sessions:%s" // sessions:jwt
	KeyStatRoot       = "state_root"
)

type ProxyResult struct {
	Type       string            `json:"type"`
	DeviceID   string            `json:"device_id,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	BodyBase64 string            `json:"body_base64,omitempty"`
	Bytes      int64             `json:"bytes,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type Hub struct {
	clients       map[string]*Client
	register      chan *Client
	unregister    chan *Client
	resultWaiters map[string]chan ProxyResult
	lastResults   map[string]ProxyResult
	rc            *redis.Client
	mapWaiters    map[string]chan MapEvent
	mapMu         sync.RWMutex
	mu            sync.RWMutex
	globalPause   bool // Circuit Breaker state
	OnOracleProposal func(models.OracleProposal)
}

func NewHub() *Hub {
	h := &Hub{
		clients:       make(map[string]*Client),
		register:      make(chan *Client, 16384),
		unregister:    make(chan *Client, 16384),
		resultWaiters: make(map[string]chan ProxyResult),
		lastResults:   make(map[string]ProxyResult),
		mapWaiters:    make(map[string]chan MapEvent),
	}
	go h.cleanupLoop()
	return h
}

func (h *Hub) InitRedis(redisURL string) {
	if redisURL == "" {
		log.Println("Redis URL empty, Hub running in local-only memory mode")
		return
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("Failed to parse Redis URL: %v", err)
		return
	}
	h.rc = redis.NewClient(opts)
	if err := h.rc.Ping(context.Background()).Err(); err != nil {
		log.Printf("Redis ping failed: %v", err)
		h.rc = nil
		return
	}
	log.Println("Redis connected. Hub horizontal scaling enabled.")
	go h.subscribeRedisProxyStart()
	go h.subscribeRedisProxyOpen()
	go h.subscribeRedisProxyResult()
	go h.subscribeRedisComputeTask()
	go h.subscribeRedisLinkRequest()
	go h.subscribeRedisOracleProposal()
	go h.subscribeRedisFeederAudit()
}

func (h *Hub) IsRedisEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rc != nil
}

func (h *Hub) PingRedis(ctx context.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.rc == nil {
		return nil
	}
	return h.rc.Ping(ctx).Err()
}

type ProxyCmd struct {
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id"`
}

func (h *Hub) subscribeRedisProxyStart() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:proxy_cmd")
	for msg := range pubsub.Channel() {
		var cmd ProxyCmd
		if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
			continue
		}
		if client, ok := h.GetClient(cmd.DeviceID); ok {
			payload, _ := json.Marshal(map[string]string{
				"type":       "proxy_start",
				"session_id": cmd.SessionID,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("[Redis] proxy_start dropped for device_id=%s queue full", cmd.DeviceID)
			}
		}
	}
}

func (h *Hub) subscribeRedisProxyOpen() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:proxy_open")
	for msg := range pubsub.Channel() {
		var cmd struct {
			DeviceID   string `json:"device_id"`
			SessionID  string `json:"session_id"`
			TargetHost string `json:"target_host"`
			TargetPort int    `json:"target_port"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
			continue
		}
		if client, ok := h.GetClient(cmd.DeviceID); ok {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":        "proxy_open",
				"session_id":  cmd.SessionID,
				"target_host": cmd.TargetHost,
				"target_port": cmd.TargetPort,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("[Redis] proxy_open dropped for device_id=%s queue full", cmd.DeviceID)
			}
		}
	}
}

func (h *Hub) BroadcastProxyOpen(deviceID, sessionID, targetHost string, targetPort int) {
	if h.rc != nil {
		cmd := map[string]interface{}{
			"device_id":   deviceID,
			"session_id":  sessionID,
			"target_host": targetHost,
			"target_port": targetPort,
		}
		b, _ := json.Marshal(cmd)
		h.rc.Publish(context.Background(), "Exra:proxy_open", b)
	} else {
		if client, ok := h.GetClient(deviceID); ok {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":        "proxy_open",
				"session_id":  sessionID,
				"target_host": targetHost,
				"target_port": targetPort,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("proxy_open dropped for device_id=%s (queue full)", deviceID)
			}
		}
	}
}

func (h *Hub) BroadcastLinkRequest(deviceID, tgUser, tgName, requestId string) {
	if h.rc != nil {
		cmd := map[string]string{
			"device_id":     deviceID,
			"tg_user":       tgUser,
			"tg_first_name": tgName,
			"request_id":    requestId,
		}
		b, _ := json.Marshal(cmd)
		h.rc.Publish(context.Background(), "Exra:link_request", b)
	} else {
		if client, ok := h.GetClient(deviceID); ok {
			payload, _ := json.Marshal(map[string]string{
				"type":          "link_request",
				"tg_user":       tgUser,
				"tg_first_name": tgName,
				"request_id":    requestId,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("link_request dropped for device_id=%s (queue full)", deviceID)
			}
		}
	}
}

func (h *Hub) subscribeRedisProxyResult() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:proxy_result")
	for msg := range pubsub.Channel() {
		var res ProxyResult
		if err := json.Unmarshal([]byte(msg.Payload), &res); err != nil {
			continue
		}
		// If someone locally is waiting for this Result, serve it
		h.mu.RLock()
		ch, ok := h.resultWaiters[res.SessionID]
		h.mu.RUnlock()

		if ok {
			select {
			case ch <- res:
			default:
			}
		}
		h.mu.Lock()
		h.lastResults[res.SessionID] = res
		h.mu.Unlock()
	}
}

func (h *Hub) subscribeRedisComputeTask() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:compute_task")
	for msg := range pubsub.Channel() {
		var cmd struct {
			NodeID string             `json:"node_id"`
			Task   models.ComputeTask `json:"task"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
			continue
		}
		if client, ok := h.GetClient(cmd.NodeID); ok {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":       "compute_task",
				"task_id":    cmd.Task.ID,
				"task_type":  cmd.Task.TaskType,
				"input_url":  cmd.Task.InputURL,
				"requirements": cmd.Task.Requirements,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("[Redis] compute_task dropped for device_id=%s queue full", cmd.NodeID)
			}
		}
	}
}


func (h *Hub) subscribeRedisLinkRequest() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:link_request")
	for msg := range pubsub.Channel() {
		var cmd struct {
			DeviceID    string `json:"device_id"`
			TgUser      string `json:"tg_user"`
			TgFirstName string `json:"tg_first_name"`
			RequestID   string `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
			continue
		}
		if client, ok := h.GetClient(cmd.DeviceID); ok {
			payload, _ := json.Marshal(map[string]string{
				"type":          "link_request",
				"tg_user":       cmd.TgUser,
				"tg_first_name": cmd.TgFirstName,
				"request_id":    cmd.RequestID,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("[Redis] link_request dropped for device_id=%s queue full", cmd.DeviceID)
			}
		}
	}
}

func (h *Hub) SetGlobalPause(active bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.globalPause = active
	if active {
		log.Println("[CircuitBreaker] Global Pause ACTIVE. Blocking new sessions.")
	} else {
		log.Println("[CircuitBreaker] Global Pause DEACTIVATED.")
	}
}

func (h *Hub) IsGlobalPause() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.globalPause
}

func (h *Hub) subscribeRedisOracleProposal() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:oracle_proposal")
	for msg := range pubsub.Channel() {
		var prop models.OracleProposal
		if err := json.Unmarshal([]byte(msg.Payload), &prop); err != nil {
			continue
		}
		// Dispatch to oracle logic if callback is set
		if h.OnOracleProposal != nil {
			h.OnOracleProposal(prop)
		}
	}
}

func (h *Hub) BroadcastOracleProposal(prop models.OracleProposal) {
	if h.rc != nil {
		b, _ := json.Marshal(prop)
		h.rc.Publish(context.Background(), "Exra:oracle_proposal", b)
	} else {
		// Local-only: immediate self-delivery
		if h.OnOracleProposal != nil {
			h.OnOracleProposal(prop)
		}
	}
}

func (h *Hub) BroadcastComputeTask(nodeID string, task *models.ComputeTask) {
	if h.rc != nil {
		cmd := struct {
			NodeID string             `json:"node_id"`
			Task   models.ComputeTask `json:"task"`
		}{NodeID: nodeID, Task: *task}
		b, _ := json.Marshal(cmd)
		h.rc.Publish(context.Background(), "Exra:compute_task", b)
	} else {
		// Fallback to local
		if client, ok := h.GetClient(nodeID); ok {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":       "compute_task",
				"task_id":    task.ID,
				"task_type":  task.TaskType,
				"input_url":  task.InputURL,
				"requirements": task.Requirements,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("compute_task dropped for device_id=%s (queue full)", nodeID)
			}
		}
	}
}

func (h *Hub) subscribeRedisFeederAudit() {
	pubsub := h.rc.Subscribe(context.Background(), "Exra:feeder_audit")
	for msg := range pubsub.Channel() {
		var cmd struct {
			FeederID     string `json:"feeder_id"`
			AssignmentID int64  `json:"assignment_id"`
			TargetIP     string `json:"target_ip"`
			TargetPort   int    `json:"target_port"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
			continue
		}
		if client, ok := h.GetClient(cmd.FeederID); ok {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":          "feeder_audit",
				"assignment_id": cmd.AssignmentID,
				"target_ip":     cmd.TargetIP,
				"target_port":   cmd.TargetPort,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("[Redis] feeder_audit dropped for device_id=%s queue full", cmd.FeederID)
			}
		}
	}
}

func (h *Hub) BroadcastFeederTask(feederID string, assignmentID int64, targetIP string, targetPort int) {
	if h.rc != nil {
		cmd := map[string]interface{}{
			"feeder_id":     feederID,
			"assignment_id": assignmentID,
			"target_ip":     targetIP,
			"target_port":   targetPort,
		}
		b, _ := json.Marshal(cmd)
		h.rc.Publish(context.Background(), "Exra:feeder_audit", b)
	} else {
		if client, ok := h.GetClient(feederID); ok {
			payload, _ := json.Marshal(map[string]interface{}{
				"type":          "feeder_audit",
				"assignment_id": assignmentID,
				"target_ip":     targetIP,
				"target_port":   targetPort,
			})
			select {
			case client.Send <- payload:
			default:
				log.Printf("feeder_audit dropped for device_id=%s (queue full)", feederID)
			}
		}
	}
}

func (h *Hub) BroadcastProxyResult(res *ProxyResult) {
	if h.rc != nil {
		b, _ := json.Marshal(res)
		h.rc.Publish(context.Background(), "Exra:proxy_result", b)
	}
	
	// Deliver locally too in case the waiter is on this same instance
	h.mu.RLock()
	ch, ok := h.resultWaiters[res.SessionID]
	h.mu.RUnlock()

	if ok {
		select {
		case ch <- *res:
		default:
		}
	}
	h.mu.Lock()
	h.lastResults[res.SessionID] = *res
	h.mu.Unlock()
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if client.DeviceID != "" {
				if _, exists := h.clients[client.DeviceID]; !exists {
					metrics.ActiveNodes.Inc()
				}
				h.clients[client.DeviceID] = client
			}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			current, ok := h.clients[client.DeviceID]
			if ok && current == client {
				delete(h.clients, client.DeviceID)
				metrics.ActiveNodes.Dec()
				close(client.Send)
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		h.mu.Lock()
		// Clear stale proxy results to prevent memory leak
		// In production, we would use a proper TTL/LRU, but for this scale
		// regular purging of the map is sufficient.
		if len(h.lastResults) > 5000 {
			h.lastResults = make(map[string]ProxyResult)
			log.Printf("[Hub] Purged %d stale proxy results from memory", 5000)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) GetClient(deviceID string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clients[deviceID]
	return client, ok
}

func (h *Hub) BindClientDeviceID(client *Client, deviceID string) {
	h.mu.Lock()
	if client.DeviceID != "" {
		if existing, ok := h.clients[client.DeviceID]; ok && existing == client {
			delete(h.clients, client.DeviceID)
		}
	}
	client.DeviceID = deviceID
	client.Lat, client.Lng = ResolveCoords(client.IP)
	h.clients[deviceID] = client
	h.mu.Unlock()

	// Broadcast burst effect for the neon map
	h.BroadcastMapEvent(MapEvent{
		Type:     "burst",
		DeviceID: deviceID,
		Lat:      client.Lat,
		Lng:      client.Lng,
		Country:  client.Country,
	})
}

func (h *Hub) AwaitProxyResult(sessionID string, timeout time.Duration) (ProxyResult, bool) {
	ch := make(chan ProxyResult, 1)
	h.mu.Lock()
	h.resultWaiters[sessionID] = ch
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.resultWaiters, sessionID)
		h.mu.Unlock()
	}()

	select {
	case res := <-ch:
		return res, true
	case <-time.After(timeout):
		return ProxyResult{}, false
	}
}

func (h *Hub) HandleProxyResult(res ProxyResult) {
	h.mu.Lock()
	h.lastResults[res.SessionID] = res
	if ch, ok := h.resultWaiters[res.SessionID]; ok {
		select {
		case ch <- res:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) GetLastProxyResult(sessionID string) (ProxyResult, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	res, ok := h.lastResults[sessionID]
	return res, ok
}

func DecodeResultBody(bodyBase64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(bodyBase64)
}

func (h *Hub) BroadcastMapEvent(event MapEvent) {
	h.mapMu.RLock()
	defer h.mapMu.RUnlock()
	for _, ch := range h.mapWaiters {
		select {
		case ch <- event:
		default:
		}
	}
	
	if h.rc != nil {
		b, _ := json.Marshal(event)
		h.rc.Publish(context.Background(), "Exra:map_event", b)
	}
}

func (h *Hub) ListenMapEvents(id string) chan MapEvent {
	ch := make(chan MapEvent, 128)
	h.mapMu.Lock()
	h.mapWaiters[id] = ch
	h.mapMu.Unlock()
	return ch
}

func (h *Hub) StopMapEvents(id string) {
	h.mapMu.Lock()
	delete(h.mapWaiters, id)
	h.mapMu.Unlock()
}

// ── Marketplace v2.1 Redis Operations ──

// SyncNodeToRedis updates the discovery ZSET for a node.
// Score is currently PriceGB (lower is better for discovery sort).
func (h *Hub) SyncNodeToRedis(ctx context.Context, country, rsTier string, score float64, pubNodeJSON string) error {
	if h.rc == nil {
		return nil
	}
	key := fmt.Sprintf(KeyPrefixNodes, country, rsTier)
	return h.rc.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: pubNodeJSON,
	}).Err()
}

// GetDiscoveryNodes returns top nodes from the ZSET.
func (h *Hub) GetDiscoveryNodes(ctx context.Context, country, rsTier string, count int64) ([]string, error) {
	if h.rc == nil {
		return nil, nil
	}
	key := fmt.Sprintf(KeyPrefixNodes, country, rsTier)
	return h.rc.ZRange(ctx, key, 0, count-1).Result()
}

// RemoveNodeFromRedis removes a node from the discovery ZSET (e.g. slots full).
func (h *Hub) RemoveNodeFromRedis(ctx context.Context, country, rsTier string, pubNodeJSON string) error {
	if h.rc == nil {
		return nil
	}
	key := fmt.Sprintf(KeyPrefixNodes, country, rsTier)
	return h.rc.ZRem(ctx, key, pubNodeJSON).Err()
}

// CreateSessionInRedis initializes a micro-billing session.
func (h *Hub) CreateSessionInRedis(ctx context.Context, jwt string, data map[string]interface{}) error {
	if h.rc == nil {
		return nil
	}
	key := fmt.Sprintf(KeyPrefixSessions, jwt)
	return h.rc.HSet(ctx, key, data).Err()
}

// DeductSessionBalance atomically updates session usage.
func (h *Hub) DeductSessionBalance(ctx context.Context, jwt string, cost float64) (float64, error) {
	if h.rc == nil {
		return 0, nil
	}
	key := fmt.Sprintf(KeyPrefixSessions, jwt)
	// cost is deducted from 'credits' field
	return h.rc.HIncrByFloat(ctx, key, "credits", -cost).Result()
}

