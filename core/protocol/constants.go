// Package protocol
// Author: momentics <momentics@gmail.com>
//
// WebSocket wire protocol constants

package protocol

const (
	// Control opcodes (<0x8)
	OpcodeContinuation = 0x0
	OpcodeText         = 0x1
	OpcodeBinary       = 0x2
	OpcodeClose        = 0x8
	OpcodePing         = 0x9
	OpcodePong         = 0xA

	// Frame limit settings
	MaxControlPayloadLen = 125
	MaxFrameHeaderLen    = 14 // for extended payloads with masking

	// Bit masks
	FinBit  = 0x80
	MaskBit = 0x80

	// Close codes
	CloseNormalClosure      = 1000
	CloseGoingAway          = 1001
	CloseProtocolError      = 1002
	CloseUnsupportedData    = 1003
	CloseNoStatusRcvd       = 1005
	CloseAbnormalClosure    = 1006
	CloseInvalidPayloadData = 1007
	ClosePolicyViolation    = 1008
	CloseMessageTooBig      = 1009
	CloseMissingExtension   = 1010
	CloseInternalServerErr  = 1011
)
