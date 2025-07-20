// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// facade_wsconnection_test.go — Тесты на создание WSConnection через facade.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/protocol"
)

func TestFacade_WSConnection(t *testing.T) {
	h, err := facade.New(facade.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create HioloadWS: %v", err)
	}
	conn := h.CreateWebSocketConnection()
	if conn == nil {
		t.Fatal("CreateWebSocketConnection returned nil")
	}
	conn.SetHandler(protocol.HandlerFunc(func(data any) error {
		return nil
	}))
	go conn.Start()
	_ = conn.Send(&protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeText,
		PayloadLen: 4,
		Payload:    []byte("ping"),
	})
	conn.Close()
}
