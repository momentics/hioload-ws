// File: protocol/frame_codec.go
// Package protocol implements zero-copy frame codec with frame size enforcement.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Implements WebSocket frame encoding/decoding with payload size limits
// to prevent resource exhaustion in high-load scenarios.

package protocol

import (
	"encoding/binary"
	"errors"
)

// MaxFramePayload defines the maximum allowed payload size for a single frame.
// This limit protects against excessively large frames that could exhaust memory.
const MaxFramePayload = 1 << 20 // 1 MiB

// DecodeFrameFromBytes parses raw WebSocket frame into WSFrame,
// enforcing maximum payload size.
func DecodeFrameFromBytes(raw []byte) (*WSFrame, error) {
	if len(raw) < 2 {
		return nil, errors.New("frame too short")
	}
	fin := raw[0]&0x80 != 0
	opcode := raw[0] & 0x0F
	masked := raw[1]&0x80 != 0
	length := int64(raw[1] & 0x7F)
	offset := 2

	switch length {
	case 126:
		if len(raw) < offset+2 {
			return nil, errors.New("frame too short for extended payload length")
		}
		length = int64(binary.BigEndian.Uint16(raw[offset:]))
		offset += 2
	case 127:
		if len(raw) < offset+8 {
			return nil, errors.New("frame too short for extended payload length")
		}
		length = int64(binary.BigEndian.Uint64(raw[offset:]))
		offset += 8
	}

	if length > MaxFramePayload {
		return nil, errors.New("frame payload exceeds maximum allowed size")
	}

	var maskKey [4]byte
	if masked {
		if len(raw) < offset+4 {
			return nil, errors.New("frame too short for mask key")
		}
		copy(maskKey[:], raw[offset:offset+4])
		offset += 4
	}

	if int64(len(raw[offset:])) < length {
		return nil, errors.New("payload truncated")
	}
	payloadData := raw[offset : offset+int(length)]

	payload := make([]byte, length)
	if masked {
		for i := int64(0); i < length; i++ {
			payload[i] = payloadData[i] ^ maskKey[i%4]
		}
	} else {
		copy(payload, payloadData)
	}

	return &WSFrame{
		IsFinal:    fin,
		Opcode:     opcode,
		Masked:     masked,
		PayloadLen: length,
		MaskKey:    maskKey,
		Payload:    payload,
	}, nil
}

// EncodeFrameToBytes serializes WSFrame into []byte,
// enforcing maximum payload size.
func EncodeFrameToBytes(f *WSFrame) ([]byte, error) {
	if f.PayloadLen > MaxFramePayload {
		return nil, errors.New("frame payload exceeds maximum allowed size")
	}
	b0 := byte(0x80) | (f.Opcode & 0x0F)
	plen := int(f.PayloadLen)
	var hdr []byte

	switch {
	case plen <= 125:
		hdr = []byte{b0, byte(plen)}
	case plen <= 0xFFFF:
		hdr = make([]byte, 4)
		hdr[0] = b0
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:], uint16(plen))
	default:
		hdr = make([]byte, 10)
		hdr[0] = b0
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:], uint64(plen))
	}

	buf := make([]byte, len(hdr)+plen)
	copy(buf, hdr)
	copy(buf[len(hdr):], f.Payload)
	return buf, nil
}
