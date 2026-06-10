package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

func TestRPCRequiresAPIKey(t *testing.T) {
	app := testApp(t, testConfigWithKeys(1, 1))
	req := httptest.NewRequest(http.MethodPost, "/", body(`{"jsonrpc":"2.0","id":1,"method":"getHealth"}`))
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRPCForwardsHeliusCompatibleAPIKey(t *testing.T) {
	var seenAPIKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAPIKey = r.URL.Query().Get("api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
	}))
	defer upstream.Close()

	cfg := testConfigWithKeys(1, 1)
	cfg.Helius.RPCBaseURL = upstream.URL
	app := testApp(t, cfg)
	req := httptest.NewRequest(http.MethodPost, "/?api-key=client", body(`{"jsonrpc":"2.0","id":1,"method":"getHealth"}`))
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if seenAPIKey != "helius-1" {
		t.Fatalf("upstream api-key = %q, want helius-1", seenAPIKey)
	}
	if got := rec.Header().Get("X-Heliproxy-Key-ID"); got != "a" {
		t.Fatalf("X-Heliproxy-Key-ID = %q, want a", got)
	}
}

func TestRPCFailoverOnRateLimit(t *testing.T) {
	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		switch r.URL.Query().Get("api-key") {
		case "helius-1":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
		case "helius-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok-from-2"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer upstream.Close()

	cfg := testConfigWithKeys(2, 1)
	cfg.Helius.RPCBaseURL = upstream.URL
	app := testApp(t, cfg)
	req := httptest.NewRequest(http.MethodPost, "/?api-key=client", body(`{"jsonrpc":"2.0","id":1,"method":"getHealth"}`))
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["result"] != "ok-from-2" {
		t.Fatalf("result = %v, want ok-from-2", payload["result"])
	}
}

func TestRPCDoesNotFailoverOnJSONRPCErrorHTTP200(t *testing.T) {
	var calls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"bad params"}}`))
	}))
	defer upstream.Close()

	cfg := testConfigWithKeys(2, 1)
	cfg.Helius.RPCBaseURL = upstream.URL
	app := testApp(t, cfg)
	req := httptest.NewRequest(http.MethodPost, "/?api-key=client", body(`{"jsonrpc":"2.0","id":1,"method":"bad"}`))
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRESTForwardsHeliusCompatibleAPIKey(t *testing.T) {
	var seenAPIKey string
	var seenPath string
	var seenFoo string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAPIKey = r.URL.Query().Get("api-key")
		seenPath = r.URL.Path
		seenFoo = r.URL.Query().Get("foo")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balances":[],"totalUsdValue":0}`))
	}))
	defer upstream.Close()

	cfg := testConfigWithKeys(1, 1)
	cfg.Helius.RestBaseURL = upstream.URL
	app := testApp(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/v1/wallet/abc/balances?api-key=client&foo=bar", nil)
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if seenAPIKey != "helius-1" {
		t.Fatalf("upstream api-key = %q, want helius-1", seenAPIKey)
	}
	if seenPath != "/v1/wallet/abc/balances" {
		t.Fatalf("upstream path = %q", seenPath)
	}
	if seenFoo != "bar" {
		t.Fatalf("foo = %q, want bar", seenFoo)
	}
}

func TestWebSocketForwardsHeliusCompatibleAPIKey(t *testing.T) {
	var seenAPIKey string
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAPIKey = r.URL.Query().Get("api-key")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade upstream: %v", err)
			return
		}
		defer conn.Close()
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read upstream: %v", err)
			return
		}
		if err := conn.WriteMessage(messageType, payload); err != nil {
			t.Errorf("write upstream: %v", err)
		}
	}))
	defer upstream.Close()

	cfg := testConfigWithKeys(1, 1)
	cfg.Helius.WSBaseURL = "ws" + strings.TrimPrefix(upstream.URL, "http")
	app := testApp(t, cfg)
	server := httptest.NewServer(app.WSRoutes())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/?api-key=client"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial client: %v", err)
	}
	defer client.Close()
	if err := client.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"2.0","id":1,"method":"slotSubscribe"}`)); err != nil {
		t.Fatalf("write client: %v", err)
	}
	_, payload, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read client: %v", err)
	}
	if !strings.Contains(string(payload), "slotSubscribe") {
		t.Fatalf("payload = %s", payload)
	}
	if seenAPIKey != "helius-1" {
		t.Fatalf("upstream api-key = %q, want helius-1", seenAPIKey)
	}
}

func TestHealthIsUnauthenticated(t *testing.T) {
	app := testApp(t, testConfigWithKeys(1, 1))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	app.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func testApp(t *testing.T, cfg *Config) *App {
	t.Helper()
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.yaml"))
	return NewApp(store, cfg)
}

func body(s string) *strings.Reader { return strings.NewReader(s) }
