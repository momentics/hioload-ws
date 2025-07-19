// Package tests
// Author: momentics <momentics@gmail.com>
//
// Integration tests for hioload-ws ensuring proper layer interactions.

package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/internal/websocket"
)

// TestCompleteWebSocketFlow tests the WebSocket handshake and message loop.
func TestCompleteWebSocketFlow(t *testing.T) {
	config := facade.DefaultConfig()
	config.NumWorkers = 2
	hioload, err := facade.New(config)
	if err != nil {
		t.Fatal(err)
	}
	defer hioload.Stop()
	if err := hioload.Start(); err != nil {
		t.Fatal(err)
	}

	upgrader := websocket.NewUpgrader(hioload.GetTransport(), hioload.GetBufferPool())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		conn.Start()
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Expected status %d, got %d", http.StatusSwitchingProtocols, resp.StatusCode)
	}
	if resp.Header.Get("Upgrade") != "websocket" {
		t.Error("Missing Upgrade header")
	}
}
