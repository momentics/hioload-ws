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
// DecodeFrameFromBytes parses raw WebSocket frame into WSFrame,
// enforcing maximum payload size.
// Returns frame, consumed bytes, and error.
// If frame is incomplete, returns (nil, 0, nil).
func DecodeFrameFromBytes(raw []byte) (*WSFrame, int, error) {
	if len(raw) < 2 {
		return nil, 0, nil // Incomplete
	}
	fin := raw[0]&0x80 != 0
	opcode := raw[0] & 0x0F
	masked := raw[1]&0x80 != 0
	length := int64(raw[1] & 0x7F)
	offset := 2

	switch length {
	case 126:
		if len(raw) < offset+2 {
			return nil, 0, nil // Incomplete
		}
		length = int64(binary.BigEndian.Uint16(raw[offset:]))
		offset += 2
	case 127:
		if len(raw) < offset+8 {
			return nil, 0, nil // Incomplete
		}
		length = int64(binary.BigEndian.Uint64(raw[offset:]))
		offset += 8
	}

	if length > MaxFramePayload {
		return nil, 0, errors.New("frame payload exceeds maximum allowed size")
	}

	var maskKey [4]byte
	if masked {
		if len(raw) < offset+4 {
			return nil, 0, nil // Incomplete
		}
		copy(maskKey[:], raw[offset:offset+4])
		offset += 4
	}

	totalLen := offset + int(length)
	if len(raw) < totalLen {
		return nil, 0, nil // Incomplete
	}

	payloadData := raw[offset:totalLen]
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
	}, totalLen, nil
}

// EncodeFrameToBytes serializes WSFrame into []byte,
// enforcing maximum payload size.
func EncodeFrameToBytes(f *WSFrame) ([]byte, error) {
	return EncodeFrameToBytesWithMask(f, f.Masked)
}

// EncodeFrameToBufferWithMask serializes WSFrame into a caller-managed buffer,
// minimizing allocations. Returned slice aliases dst.
func EncodeFrameToBufferWithMask(f *WSFrame, mask bool, dst []byte) ([]byte, error) {
	if f.PayloadLen > MaxFramePayload {
		return nil, errors.New("frame payload exceeds maximum allowed size")
	}

	var b0 byte
	if f.IsFinal {
		b0 = 0x80
	}
	b0 |= (f.Opcode & 0x0F)

	plen := int(f.PayloadLen)
	var hdr [10]byte
	var header []byte

	switch {
	case plen <= 125:
		header = hdr[:2]
		header[0] = b0
		if mask {
			header[1] = byte(plen) | 0x80 // Set mask bit
		} else {
			header[1] = byte(plen)
		}
	case plen <= 0xFFFF:
		header = hdr[:4]
		header[0] = b0
		if mask {
			header[1] = 126 | 0x80 // Set mask bit
		} else {
			header[1] = 126
		}
		binary.BigEndian.PutUint16(header[2:], uint16(plen))
	default:
		header = hdr[:10]
		header[0] = b0
		if mask {
			header[1] = 127 | 0x80 // Set mask bit
		} else {
			header[1] = 127
		}
		binary.BigEndian.PutUint64(header[2:], uint64(plen))
	}

	dst = append(dst[:0], header...)
	if mask {
		maskKey := [4]byte{0x12, 0x34, 0x56, 0x78} // Example mask key
		dst = append(dst, maskKey[:]...)
	}

	start := len(dst)
	dst = append(dst, f.Payload...)
	if mask {
		for i := 0; i < plen; i++ {
			dst[start+i] ^= dst[len(header)+(i%4)]
		}
	}

	return dst, nil
}

// EncodeFrameToBytesWithMask serializes WSFrame into []byte with specific mask setting,
// enforcing maximum payload size.
func EncodeFrameToBytesWithMask(f *WSFrame, mask bool) ([]byte, error) {
	return EncodeFrameToBufferWithMask(f, mask, nil)
}
