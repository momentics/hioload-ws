// File: internal/websocket/connection.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Connection adapts protocol.WSConnection to an api.Handler-driven API.
// Implements zero-copy receive and configurable timeouts on both read and write operations.
// Relies on WSConnection exposing Transport() and RecvZeroCopy() for direct buffer access.

package websocket

import (
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// Connection wraps a protocol.WSConnection and dispatches frames to an api.Handler,
// applying read/write deadlines for robust I/O control.
type Connection struct {
	wsConn       *protocol.WSConnection // Underlying WebSocket connection
	handler      api.Handler            // Application-level message handler
	readTimeout  time.Duration          // Deadline for read operations
	writeTimeout time.Duration          // Deadline for write operations
}

// NewConnection constructs a new Connection wrapper with specified timeouts.
// readTimeout and writeTimeout bound each blocking I/O call to prevent hangs.
func NewConnection(ws *protocol.WSConnection, readTimeout, writeTimeout time.Duration) *Connection {
	return &Connection{
		wsConn:       ws,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

// SetHandler assigns the api.Handler to process incoming frames.
func (c *Connection) SetHandler(h api.Handler) {
	c.handler = h
}

// Start launches the internal receive and keep-alive loops.
// wsConn.Start() must be called prior to launching loops.
func (c *Connection) Start() {
	c.wsConn.Start()
	go c.messageLoop()
	go c.keepAlive()
}

// messageLoop continuously reads frames via zero-copy into pooled buffers,
// applies read deadlines, and dispatches each buffer to the handler.
func (c *Connection) messageLoop() {
	defer c.wsConn.Close()
	for {
		// Apply read deadline if transport supports it
		if t, ok := c.wsConn.Transport().(interface{ SetReadDeadline(time.Time) error }); ok {
			t.SetReadDeadline(time.Now().Add(c.readTimeout))
		}

		// Zero-copy receive returns []api.Buffer
		bufs, err := c.wsConn.RecvZeroCopy()
		if err != nil {
			// Exit loop on error or closure
			return
		}

		// Dispatch each buffer to the handler
		for _, buf := range bufs {
			if c.handler != nil {
				c.handler.Handle(buf)
			}
			buf.Release()
		}
	}
}

// keepAlive sends periodic ping frames to maintain the WebSocket connection,
// applying write deadlines to bound each send call.
func (c *Connection) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if t, ok := c.wsConn.Transport().(interface{ SetWriteDeadline(time.Time) error }); ok {
			t.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		}
		c.wsConn.SendFrame(&protocol.WSFrame{Opcode: protocol.OpcodePing})
	}
}
