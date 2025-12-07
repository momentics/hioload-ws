package protocol_test

import (
	"bytes"
	"testing"

	"github.com/momentics/hioload-ws/protocol"
)

func TestEncodeDecodeFrame(t *testing.T) {
	payload := []byte("hello")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeText,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
	}
	data, err := protocol.EncodeFrameToBytes(frame)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := protocol.DecodeFrameFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Error("Payload mismatch")
	}
}
