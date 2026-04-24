package tests

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"exra/db"
	"exra/handlers"
	"exra/hub"
	"exra/models"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ChainSafe/go-schnorrkel"
	"github.com/gorilla/websocket"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestMarketplaceIntegration(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer mockDB.Close()
	
	oldDB := db.DB
	db.DB = mockDB
	defer func() { db.DB = oldDB }()

	// Expectations for WS registration and result processing
	// 1. UpsertWSNode (INSERT ... ON CONFLICT ... RETURNING 23 columns)
	mock.ExpectQuery("INSERT INTO nodes").WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "device_id", "public_key", "ip", "address", "port", "country",
			"device_type", "device_tier", "is_residential", "asn_org", "status",
			"traffic_bytes", "bandwidth_mbps",
			"cpu_model", "cpu_cores", "vram_mb", "ram_mb",
			"did", "identity_tier",
			"active", "price_per_gb", "auto_price",
			"last_seen", "last_heartbeat", "created_at",
		}).AddRow(
			"node-uuid", "test-worker-1", "pubkey-hex", "127.0.0.1", "", 0, "",
			"amd64", "compute", true, "", "online",
			0, 0,
			"", 0, 0, 0,
			"did:peaq:test-worker-1", "verified",
			true, 1.50, true,
			time.Now(), time.Now(), time.Now(),
		))

	// 2. CompleteTask (called when worker sends "compute_result")
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE task_assignments").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE compute_tasks").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT reward_usd").WillReturnRows(sqlmock.NewRows([]string{"reward_usd"}).AddRow(0.0)) 
	mock.ExpectCommit()

	// 3. SetNodeOfflineByDeviceID (called on WS close) — now transactional (Fix #6)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE nodes").WithArgs("test-worker-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE worker_listings").WithArgs("test-worker-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	// 1. Setup Hub and Handlers
	h := hub.NewHub()
	go h.Run()
	handlers.SetHub(h)

	// We need a buyer with balance.
	// Bypassing DB for this test and using a manual context injection in middleware
	// Actually, easier to use the real handler with a mocked buyer context.

	// 2. Setup WebSocket Worker
	server := httptest.NewServer(handlers.WsHandler(h))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect ws: %v", err)
	}
	defer ws.Close()

	deviceID := "test-worker-1"
	did := "did:peaq:test-worker-1"

	// AUDIT §1 D1: ws register now requires a valid sr25519 (Schnorrkel)
	// DID signature over "deviceID:did". Generate a real keypair and sign.
	var seed [32]byte
	_, _ = rand.Read(seed[:])
	msk, err := schnorrkel.NewMiniSecretKeyFromRaw(seed)
	if err != nil {
		t.Fatalf("MiniSecretKey: %v", err)
	}
	sk := msk.ExpandEd25519()
	pk, _ := sk.Public()
	pkBytes := pk.Encode()
	pubHex := hex.EncodeToString(pkBytes[:])

	signMsg := deviceID + ":" + did
	ctx := schnorrkel.NewSigningContext([]byte("substrate"), []byte(signMsg))
	sig, err := sk.Sign(ctx)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sigBytes := sig.Encode()
	signature := hex.EncodeToString(sigBytes[:])

	regMsg, _ := json.Marshal(map[string]interface{}{
		"type":      "register",
		"device_id": deviceID,
		"did":       did,
		"pub_key":   pubHex,
		"signature": signature,
		"arch":      "amd64",
		"ram_mb":    16383,
		"has_gpu":   false,
	})
	ws.WriteMessage(websocket.TextMessage, regMsg)

	// Consume the "registered" confirmation
	_, regResp, _ := ws.ReadMessage()
	var regRespMap map[string]interface{}
	json.Unmarshal(regResp, &regRespMap)
	assert.Equal(t, "registered", regRespMap["type"])

	// Wait for registration to propagate in memory Hub
	time.Sleep(100 * time.Millisecond)

	// 3. Submit Task via HTTP
	// We mock the BuyerAuth middleware by manually setting the context if we were calling the handler directly.
	// But since we are doing integration, let's just assert the Hub side.

	task := &models.ComputeTask{
		ID:       "task-test-id",
		TaskType: "dummy",
		InputURL: "http://input",
		Requirements: json.RawMessage(`{"gpu":true}`),
	}

	// Manually trigger broadcast to avoid DB dependency in this quick integration test.
	h.BroadcastComputeTask(deviceID, task)

	// 4. Verify Worker receives task
	_, msg, err := ws.ReadMessage()
	assert.NoError(t, err)
	
	var received map[string]interface{}
	json.Unmarshal(msg, &received)
	assert.Equal(t, "compute_task", received["type"])
	assert.Equal(t, "task-test-id", received["task_id"])

	// 5. Worker sends result
	resMsg, _ := json.Marshal(map[string]interface{}{
		"type":        "compute_result",
		"task_id":     "task-test-id",
		"result_hash": "hash-abc",
	})
	ws.WriteMessage(websocket.TextMessage, resMsg)

	// Sleep to let ReadPump process it
	time.Sleep(100 * time.Millisecond)

	// Verification of DB update would happen here if DB was connected.
	// For this unit-integration, we've proven the WS duplex channel works for tasks.
}

