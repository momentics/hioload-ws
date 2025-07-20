// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// websocket_handler_concurrent_test.go — Проверяет обработку входящих сообщений WSConnection.
package tests

import (
	"sync"
	"testing"
	"time"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/protocol"
)

func TestWSConnection_HandlerConcurrency(t *testing.T) {
	hioload, err := facade.New(facade.DefaultConfig())
	if err != nil {
		t.Fatalf("Create facade: %v", err)
	}
	conn := hioload.CreateWebSocketConnection()
	var mu sync.Mutex
	count := 0
	conn.SetHandler(protocol.HandlerFunc(func(data any) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}))
	go conn.Start()
	// Имитируем несколько сообщений в канал inbox
	for i := 0; i < 30; i++ {
		_ = conn.Send(&protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeBinary,
			PayloadLen: 2,
			Payload:    []byte{1, 2},
		})
	}
	time.Sleep(100 * time.Millisecond)
	conn.Close()
	if count == 0 {
		t.Error("Handler did not process any frames")
	}
}
