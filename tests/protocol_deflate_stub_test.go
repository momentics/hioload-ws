// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// protocol_deflate_stub_test.go — Placeholder for permessage-deflate extension.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/protocol"
)

func TestDeflateStub_API(t *testing.T) {
	// The test checks that the codec can in future add "deflate" without panic.
	f := &protocol.WSFrame{
		IsFinal: true, Opcode: protocol.OpcodeBinary,
		Payload: []byte("compress this"),
		PayloadLen: int64(len("compress this")),
	}
	frameBytes := protocol.EncodeFrameToBytes(f)
	decoded, err := protocol.DecodeFrameFromBytes(frameBytes)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if string(decoded.Payload) != "compress this" {
		t.Error("Payload mismatch, deflate extension stubbed")
	}
}
