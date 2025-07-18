// File: api/websocket.go
// Author: momentics <momentics@gmail.com>
//
// Defines abstract WebSocket connection and frame interfaces for protocol pipelines.
// All zero-copy and framed messaging works on top of these entities.

package api

// WebSocketFrame represents a single WebSocket frame parsed from a stream
type WebSocketFrame interface {
	// Opcode returns the frame type (text/binary/ping/etc.)
	Opcode() byte

	// Payload returns the frame's body (can be pooled memory)
	Payload() []byte

	// IsFinal returns whether this frame ends the message
	IsFinal() bool

	// CloseFrameDetails returns code + reason if this is a CLOSE frame
	CloseFrameDetails() (code int, reason string, ok bool)
}

// WebSocketConn is a high-level frame-oriented connection interface
type WebSocketConn interface {
	// ReadFrame parses and returns the next WebSocket frame.
	ReadFrame() (WebSocketFrame, error)

	// WriteFrame sends a frame (opcode, payload, FIN-bit)
	WriteFrame(fin bool, opcode byte, payload []byte) error

	// CloseStream ends the connection cleanly
	CloseStream() error
}
