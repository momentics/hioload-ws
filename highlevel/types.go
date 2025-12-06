// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
package highlevel

// Package-level variables and types

// MessageType represents the type of a WebSocket message.
type MessageType int

const (
	// TextMessage denotes a text WebSocket message.
	TextMessage MessageType = 1
	// BinaryMessage denotes a binary WebSocket message.
	BinaryMessage MessageType = 2
	// CloseMessage denotes a close control message.
	CloseMessage MessageType = 8
	// PingMessage denotes a ping control message.
	PingMessage MessageType = 9
	// PongMessage denotes a pong control message.
	PongMessage MessageType = 10
)