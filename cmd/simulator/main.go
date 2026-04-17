package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type wsMessage struct {
	Type       string            `json:"type"`
	DeviceID   string            `json:"device_id,omitempty"`
	Country    string            `json:"country,omitempty"`
	DeviceType string            `json:"device_type,omitempty"`
	ASNOrg     string            `json:"asn_org,omitempty"`
	Bytes      int64             `json:"bytes,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`
	Method     string            `json:"method,omitempty"`
	URL        string            `json:"url,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	BodyBase64 string            `json:"body_base64,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type proxyTask struct {
	SessionID  string
	Method     string
	URL        string
	Headers    map[string]string
	BodyBase64 string
}

func main() {
	wsURL := flag.String("ws", getenv("SIM_WS_URL", "ws://localhost:8080/ws"), "WebSocket URL")
	deviceID := flag.String("device-id", getenv("SIM_DEVICE_ID", "sim-node-001"), "Node device_id")
	country := flag.String("country", getenv("SIM_COUNTRY", "IN"), "Node country")
	deviceType := flag.String("device-type", getenv("SIM_DEVICE_TYPE", "pc"), "Node device type")
	asnOrg := flag.String("asn-org", getenv("SIM_ASN_ORG", "Residential ISP"), "ASN org")
	pingEvery := flag.Duration("ping-every", 20*time.Second, "JSON ping interval")
	maxBodyRead := flag.Int64("max-body-bytes", 1024, "Max bytes in proxy response payload")
	flag.Parse()

	log.Printf("[sim] connect ws=%s device_id=%s country=%s type=%s", *wsURL, *deviceID, *country, *deviceType)
	conn, _, err := websocket.DefaultDialer.Dial(*wsURL, http.Header{"X-ASN-Org": []string{*asnOrg}})
	if err != nil {
		log.Fatalf("[sim] ws dial failed: %v", err)
	}
	defer conn.Close()

	if err := writeJSON(conn, wsMessage{
		Type:       "register",
		DeviceID:   *deviceID,
		Country:    *country,
		DeviceType: *deviceType,
		ASNOrg:     *asnOrg,
	}); err != nil {
		log.Fatalf("[sim] register send failed: %v", err)
	}
	log.Printf("[sim] -> register")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pingLoop(ctx, conn, *deviceID, *asnOrg, *pingEvery)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		log.Printf("[sim] shutdown signal")
		cancel()
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	}()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[sim] read error: %v", err)
			return
		}
		log.Printf("[sim] <- %s", string(data))

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			log.Printf("[sim] invalid json: %v", err)
			continue
		}
		msgType, _ := raw["type"].(string)
		switch msgType {
		case "ping":
			_ = writeJSON(conn, wsMessage{Type: "pong", DeviceID: *deviceID, ASNOrg: *asnOrg})
			log.Printf("[sim] -> pong")
		case "proxy_start", "proxy_task", "proxy_http":
			task := extractProxyTask(raw)
			if task.URL == "" {
				log.Printf("[sim] proxy task without URL; session=%s", task.SessionID)
				continue
			}
			runProxyTask(conn, httpClient, *deviceID, task, *maxBodyRead)
		default:
			log.Printf("[sim] unhandled type=%s", msgType)
		}
	}
}

func extractProxyTask(raw map[string]any) proxyTask {
	task := proxyTask{Method: "GET", Headers: map[string]string{}}
	if v, ok := raw["session_id"].(string); ok {
		task.SessionID = v
	}
	if v, ok := raw["url"].(string); ok {
		task.URL = v
	}
	if v, ok := raw["method"].(string); ok && v != "" {
		task.Method = strings.ToUpper(v)
	}
	if v, ok := raw["body_base64"].(string); ok {
		task.BodyBase64 = v
	}
	if h, ok := raw["headers"].(map[string]any); ok {
		for k, v := range h {
			task.Headers[k] = fmt.Sprint(v)
		}
	}
	return task
}

func runProxyTask(conn *websocket.Conn, httpClient *http.Client, deviceID string, task proxyTask, maxBody int64) {
	var body io.Reader
	if task.BodyBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(task.BodyBase64)
		if err != nil {
			_ = writeJSON(conn, wsMessage{Type: "proxy_result", DeviceID: deviceID, SessionID: task.SessionID, Error: "invalid body_base64: " + err.Error()})
			return
		}
		body = bytes.NewReader(decoded)
	}

	req, err := http.NewRequest(task.Method, task.URL, body)
	if err != nil {
		_ = writeJSON(conn, wsMessage{Type: "proxy_result", DeviceID: deviceID, SessionID: task.SessionID, Error: "request build failed: " + err.Error()})
		return
	}
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		_ = writeJSON(conn, wsMessage{Type: "proxy_result", DeviceID: deviceID, SessionID: task.SessionID, Error: "http request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	payload, byteCount, err := readResponseWithSample(resp.Body, maxBody)
	if err != nil {
		_ = writeJSON(conn, wsMessage{Type: "proxy_result", DeviceID: deviceID, SessionID: task.SessionID, Error: "read response failed: " + err.Error()})
		return
	}
	headers := map[string]string{}
	for k, vals := range resp.Header {
		headers[k] = strings.Join(vals, ", ")
	}

	result := wsMessage{
		Type:       "proxy_result",
		DeviceID:   deviceID,
		SessionID:  task.SessionID,
		StatusCode: resp.StatusCode,
		Headers:    headers,
		BodyBase64: base64.StdEncoding.EncodeToString(payload),
		Bytes:      byteCount,
	}
	if err := writeJSON(conn, wsMessage{Type: "traffic", DeviceID: deviceID, SessionID: task.SessionID, Bytes: byteCount}); err != nil {
		log.Printf("[sim] send traffic failed: %v", err)
		return
	}
	if err := writeJSON(conn, result); err != nil {
		log.Printf("[sim] send proxy_result failed: %v", err)
		return
	}
	log.Printf("[sim] proxy done session=%s method=%s url=%s status=%d bytes=%d elapsed=%s", task.SessionID, task.Method, task.URL, resp.StatusCode, byteCount, time.Since(start))
}

func readResponseWithSample(r io.Reader, maxBody int64) ([]byte, int64, error) {
	const chunkSize = 32 * 1024
	buf := make([]byte, chunkSize)
	var total int64
	var sample bytes.Buffer

	for {
		n, err := r.Read(buf)
		if n > 0 {
			total += int64(n)
			remaining := maxBody - int64(sample.Len())
			if remaining > 0 {
				toWrite := int64(n)
				if toWrite > remaining {
					toWrite = remaining
				}
				_, _ = sample.Write(buf[:toWrite])
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, total, err
		}
	}
	return sample.Bytes(), total, nil
}

func pingLoop(ctx context.Context, conn *websocket.Conn, deviceID, asnOrg string, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := writeJSON(conn, wsMessage{Type: "ping", DeviceID: deviceID, ASNOrg: asnOrg}); err != nil {
				log.Printf("[sim] ping send failed: %v", err)
				return
			}
			log.Printf("[sim] -> ping")
		}
	}
}

func writeJSON(conn *websocket.Conn, v any) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(v)
}

func getenv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
