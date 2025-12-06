// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
package highlevel

import "errors"

// Version of the hioload library
const Version = "1.0.0"

// Common error types
var (
	// ErrClosed is returned when attempting to read or write from a closed connection.
	ErrClosed = errors.New("websocket: closed")

	// ErrReadLimit is returned when the read limit is exceeded.
	ErrReadLimit = errors.New("websocket: read limit exceeded")
)