// File: internal/websocket/connection.go
// Package websocket wraps protocol.WSConnection for handler integration.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package websocket

import (
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// Connection adapts protocol.WSConnection to api.Handler interface.
type Connection struct {
	wsConn  *protocol.WSConnection
	handler api.Handler
}

// NewConnection wraps an existing WSConnection.
func NewConnection(ws *protocol.WSConnection) *Connection {
	return &Connection{wsConn: ws}
}

// SetHandler assigns a high-level handler for incoming buffer data.
func (c *Connection) SetHandler(h api.Handler) {
	c.handler = h
}

// Start begins message and keep-alive loops.
func (c *Connection) Start() {
	c.wsConn.Start()
	go c.messageLoop()
	go c.keepAlive()
}

func (c *Connection) messageLoop() {
	defer c.wsConn.Close()
	for {
		bufs, err := c.wsConn.Recv()
		if err != nil {
			return
		}
		for _, buf := range bufs {
			if c.handler != nil {
				c.handler.Handle(buf)
			}
			buf.Release()
		}
	}
}

func (c *Connection) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.wsConn.Send(&protocol.WSFrame{Opcode: protocol.OpcodePing, Payload: []byte("ping")})
	}
}
