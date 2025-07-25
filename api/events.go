// File: api/events.go
// Package api defines core event types for hioload-ws.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

import (
	"context"
)

// OpenEvent is emitted when a new WebSocket connection is accepted.
type OpenEvent struct {
	Conn any             // underlying connection object, e.g. *protocol.WSConnection
	Ctx  context.Context // context carrying per-connection values
}

// CloseEvent is emitted when a WebSocket connection is closed.
type CloseEvent struct {
	Conn any
	Ctx  context.Context
}
