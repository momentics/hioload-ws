// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// integration_echo_test.go — End-to-end test of ws echo server using standard net/http & Gorilla WS.
package tests

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/momentics/hioload-ws/examples/echo"
)

func TestWebSocketEchoIntegration(t *testing.T) {
	server := httptest.NewServer(echoHandlerFunc())
	defer server.Close()

	wsURL := "ws" + server.URL[len("http"): ] + "/ws"
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	testMsg := "hioload-ws integration!"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(testMsg)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	_, resp, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(resp) != testMsg {
		t.Errorf("expected echo %q, got %q", testMsg, string(resp))
	}
}

func echoHandlerFunc() http.HandlerFunc {
	// Use the handler exactly as registered in echo example
	return func(w http.ResponseWriter, r *http.Request) {
		echo.MainHandler(w, r)
	}
}
