// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// protocol_frame_codec_test.go — Test WebSocket frame codec (zero-copy, masking).
package tests

import (
	"bytes"
	"testing"

	"github.com/momentics/hioload-ws/protocol"
)

// TestEncodeDecodeWSFrame — roundtrip testing of WebSocket frames.
func TestEncodeDecodeWSFrame(t *testing.T) {
	payload := []byte("hioload-ws test frame payload")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		Payload:    payload,
		PayloadLen: int64(len(payload)),
	}

	encoded := protocol.EncodeFrameToBytes(frame)
	decoded, err := protocol.DecodeFrameFromBytes(encoded)
	if err != nil {
		t.Fatalf("DecodeFrameFromBytes failed: %v", err)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Errorf("Payload mismatch, got %v, want %v", decoded.Payload, payload)
	}
	if decoded.Opcode != protocol.OpcodeBinary {
		t.Error("Opcode mismatch")
	}
}
