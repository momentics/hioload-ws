// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// protocol_upgrader_test.go — Проверка HTTP→WS апгрейда (заглушка).
package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/momentics/hioload-ws/internal/websocket"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/internal/transport"
)

func TestUpgrader_Upgrade(t *testing.T) {
	tr, _ := transport.NewTransport(2048)
	bp := pool.NewBufferPoolManager().GetPool(0)
	upg := websocket.NewUpgrader(tr, bp, 16)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upg.Upgrade(w, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()
	srv.Config.Handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusSwitchingProtocols && w.Result().StatusCode != http.StatusOK {
		t.Errorf("Upgrade status = %d, want %d", w.Result().StatusCode, http.StatusSwitchingProtocols)
	}
}
