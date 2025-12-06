// Package unit tests the protocol functionality.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package unit

import (
	"testing"

	"github.com/momentics/hioload-ws/protocol"
)

// TestWSFrame_EncodeDecode tests encoding and decoding of WebSocket frames.
func TestWSFrame_EncodeDecode(t *testing.T) {
	// Test with a simple binary frame
	payload := []byte("hello, world")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}
	
	encoded, err := protocol.EncodeFrameToBytes(frame)
	if err != nil {
		t.Fatalf("Failed to encode frame: %v", err)
	}
	
	decoded, err := protocol.DecodeFrameFromBytes(encoded)
	if err != nil {
		t.Fatalf("Failed to decode frame: %v", err)
	}
	
	if !decoded.IsFinal {
		t.Errorf("Expected decoded frame to be final")
	}
	
	if decoded.Opcode != protocol.OpcodeBinary {
		t.Errorf("Expected opcode to be binary, got %d", decoded.Opcode)
	}
	
	if decoded.PayloadLen != int64(len(payload)) {
		t.Errorf("Expected payload length %d, got %d", len(payload), decoded.PayloadLen)
	}
	
	if string(decoded.Payload) != string(payload) {
		t.Errorf("Expected payload '%s', got '%s'", string(payload), string(decoded.Payload))
	}
}

// TestWSFrame_EncodeDecode_Text tests encoding and decoding of text frames.
func TestWSFrame_EncodeDecode_Text(t *testing.T) {
	payload := []byte("hello, text")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeText,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}
	
	encoded, err := protocol.EncodeFrameToBytes(frame)
	if err != nil {
		t.Fatalf("Failed to encode frame: %v", err)
	}
	
	decoded, err := protocol.DecodeFrameFromBytes(encoded)
	if err != nil {
		t.Fatalf("Failed to decode frame: %v", err)
	}
	
	if decoded.Opcode != protocol.OpcodeText {
		t.Errorf("Expected opcode to be text, got %d", decoded.Opcode)
	}
	
	if string(decoded.Payload) != string(payload) {
		t.Errorf("Expected payload '%s', got '%s'", string(payload), string(decoded.Payload))
	}
}

// TestWSFrame_PayloadLimit tests payload size limit enforcement.
func TestWSFrame_PayloadLimit(t *testing.T) {
	// Create a payload that exceeds the maximum size
	oversizedPayload := make([]byte, protocol.MaxFramePayload+1)
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(oversizedPayload)),
		Payload:    oversizedPayload,
		Masked:     false,
	}
	
	_, err := protocol.EncodeFrameToBytes(frame)
	if err == nil {
		t.Errorf("Expected error when encoding oversized frame")
	}
	
	// Test with valid payload that's just at the limit
	validPayload := make([]byte, protocol.MaxFramePayload)
	frame2 := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(validPayload)),
		Payload:    validPayload,
		Masked:     false,
	}
	
	_, err = protocol.EncodeFrameToBytes(frame2)
	if err != nil {
		t.Errorf("Expected no error when encoding frame at limit, got %v", err)
	}
}

// TestDecodeFrameFromBytes_ShortFrame tests decoding of short frames.
func TestDecodeFrameFromBytes_ShortFrame(t *testing.T) {
	// Test with a frame that's too short to be valid
	shortFrame := []byte{0x81} // FIN bit set, text opcode, but no length/payload
	
	_, err := protocol.DecodeFrameFromBytes(shortFrame)
	if err == nil {
		t.Errorf("Expected error when decoding short frame")
	}
	
	// Test with frame that has length but no payload
	shortFrame2 := []byte{0x81, 0x05} // FIN bit set, text opcode, length 5, but no actual payload
	
	_, err = protocol.DecodeFrameFromBytes(shortFrame2)
	if err == nil {
		t.Errorf("Expected error when decoding frame with declared length but no payload")
	}
}