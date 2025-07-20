// File: internal/websocket/upgrader.go
// Package websocket implements HTTP→WebSocket upgrade and connection creation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Upgrader encapsulates the WebSocket handshake and constructs a new WSConnection.
// It delegates the handshake header logic to protocol.UpgradeToWebSocket.

package websocket

import (
	"net/http"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// Upgrader handles the HTTP to WebSocket upgrade process and returns a started WSConnection.
type Upgrader struct {
	transport   api.Transport  // underlying transport abstraction
	bufPool     api.BufferPool // zero-copy buffer pool
	channelSize int            // channel capacity for WSConnection inbox/outbox
}

// NewUpgrader constructs an Upgrader with explicit dependencies.
// transport: platform-specific Transport implementation
// bufPool: BufferPool for frame payloads
// channelSize: capacity for WSConnection channels
func NewUpgrader(
	transport api.Transport,
	bufPool api.BufferPool,
	channelSize int,
) *Upgrader {
	return &Upgrader{
		transport:   transport,
		bufPool:     bufPool,
		channelSize: channelSize,
	}
}

// Upgrade performs the WebSocket handshake by delegating to protocol.UpgradeToWebSocket,
// writes the response headers, and returns a started WSConnection.
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*protocol.WSConnection, error) {
	// Delegate handshake logic to protocol layer
	headers, err := protocol.UpgradeToWebSocket(r)
	if err != nil {
		return nil, err
	}

	// Write the handshake response headers
	for k, v := range headers {
		w.Header()[k] = v
	}
	w.WriteHeader(http.StatusSwitchingProtocols)

	// Create and start the WebSocket connection
	conn := protocol.NewWSConnection(u.transport, u.bufPool, u.channelSize)
	conn.Start()
	return conn, nil
}
