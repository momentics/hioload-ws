// File: protocol/frame_codec.go
// Package protocol implements zero-copy frame codec.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// PayloadLen unified to int64; Decode/Encode methods matched accordingly.

package protocol

import (
	"encoding/binary"
	"errors"
)

// DecodeFrameFromBytes parses raw WebSocket frame into WSFrame.
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
		length = int64(binary.BigEndian.Uint16(raw[offset:]))
		offset += 2
	case 127:
		length = int64(binary.BigEndian.Uint64(raw[offset:]))
		offset += 8
	}

	var maskKey [4]byte
	if masked {
		copy(maskKey[:], raw[offset:offset+4])
		offset += 4
	}
	if int64(len(raw[offset:])) < length {
		return nil, errors.New("payload truncated")
	}
	payloadData := raw[offset : offset+int(length)]

	var payload []byte
	if masked {
		payload = make([]byte, length)
		for i := int64(0); i < length; i++ {
			payload[i] = payloadData[i] ^ maskKey[i%4]
		}
	} else {
		payload = payloadData
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

// EncodeFrameToBytes serializes WSFrame into []byte.
func EncodeFrameToBytes(f *WSFrame) []byte {
	b0 := byte(0x80) | (f.Opcode & 0x0F)
	plen := len(f.Payload)
	var hdr []byte
	switch {
	case plen <= 125:
		hdr = []byte{b0, byte(plen)}
	case plen <= 0xFFFF:
		hdr = make([]byte, 4)
		hdr[0], hdr[1] = b0, 126
		binary.BigEndian.PutUint16(hdr[2:], uint16(plen))
	default:
		hdr = make([]byte, 10)
		hdr[0], hdr[1] = b0, 127
		binary.BigEndian.PutUint64(hdr[2:], uint64(plen))
	}
	buf := make([]byte, len(hdr)+plen)
	copy(buf, hdr)
	copy(buf[len(hdr):], f.Payload)
	return buf
}
