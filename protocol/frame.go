// Package protocol
// Author: momentics <momentics@gmail.com>
//
// WebSocket frame encoding/decoding and masking logic for high-throughput parsing.
//
// This module avoids allocations and implements zero-copy wherever possible.
// Designed for integration with buffer pools and NUMA-aware memory layers.

package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

// WSFrame represents a decoded WebSocket frame.
type WSFrame struct {
	IsFinal    bool  // FIN bit
	Opcode     byte  // Operation code
	Masked     bool  // Whether the frame was masked
	PayloadLen int64 // Actual payload length
	MaskKey    [4]byte
	Payload    []byte // Zero-copy reference (owner managed via pooling)
}

// DecodeFrame parses the WebSocket frame header and payload from stream.
func DecodeFrame(r io.Reader) (*WSFrame, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}

	isFin := hdr[0]&FinBit != 0
	opcode := hdr[0] & 0x0F
	isMasked := hdr[1]&MaskBit != 0
	payloadLen := int64(hdr[1] & 0x7F)

	switch payloadLen {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext[:]))
	}

	var maskKey [4]byte
	if isMasked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	if isMasked {
		unmaskInPlace(payload, maskKey)
	}

	return &WSFrame{
		IsFinal:    isFin,
		Opcode:     opcode,
		Masked:     isMasked,
		PayloadLen: payloadLen,
		MaskKey:    maskKey,
		Payload:    payload,
	}, nil
}

// EncodeFrame serializes a frame to buffer (caller owns output).
func EncodeFrame(dst []byte, opcode byte, payload []byte, mask bool) (int, error) {
	if len(payload) > int(^uint64(0)>>1) {
		return 0, errors.New("payload too large")
	}

	offset := 0
	dst[offset] = FinBit | opcode
	offset++

	var maskBit byte
	if mask {
		maskBit = MaskBit
	}
	payloadLen := len(payload)

	switch {
	case payloadLen <= 125:
		dst[offset] = byte(payloadLen) | maskBit
		offset++
	case payloadLen <= 0xFFFF:
		dst[offset] = 126 | maskBit
		offset++
		binary.BigEndian.PutUint16(dst[offset:], uint16(payloadLen))
		offset += 2
	default:
		dst[offset] = 127 | maskBit
		offset++
		binary.BigEndian.PutUint64(dst[offset:], uint64(payloadLen))
		offset += 8
	}

	var maskKey [4]byte
	if mask {
		copy(maskKey[:], []byte{0xDE, 0xAD, 0xBE, 0xEF}) // Example key
		copy(dst[offset:], maskKey[:])
		offset += 4
		unmaskInPlace(payload, maskKey)
	}

	copy(dst[offset:], payload)
	return offset + len(payload), nil
}

// unmaskInPlace applies XOR on payload using maskKey.
func unmaskInPlace(buf []byte, key [4]byte) {
	for i := 0; i < len(buf); i++ {
		buf[i] ^= key[i%4]
	}
}
