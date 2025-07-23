// File: internal/websocket/connection.go
// Package websocket provides a high-level WebSocket Connection wrapper.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Connection wraps protocol.WSConnection and dispatches frames to an api.Handler,
// applying read/write deadlines for robust I/O control.

package websocket

import (
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// Connection wraps a protocol.WSConnection and applies deadlines.
type Connection struct {
	wsConn       *protocol.WSConnection
	handler      api.Handler
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewConnection constructs a new Connection with specified timeouts.
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

// Start launches the internal loops.
func (c *Connection) Start() {
	c.wsConn.Start()
	go c.messageLoop()
	go c.keepAlive()
}

// messageLoop continuously reads frames via zero-copy,
// applies read deadlines, and dispatches each buffer.
func (c *Connection) messageLoop() {
	defer c.wsConn.Close()
	for {
		// If transport supports SetReadDeadline, enforce a read timeout.
		if rt, ok := c.wsConn.Transport().(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = rt.SetReadDeadline(time.Now().Add(c.readTimeout))
		}

		bufs, err := c.wsConn.RecvZeroCopy()
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

// keepAlive sends periodic ping frames, applying write deadlines.
func (c *Connection) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if wt, ok := c.wsConn.Transport().(interface{ SetWriteDeadline(time.Time) error }); ok {
			_ = wt.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		}
		c.wsConn.SendFrame(&protocol.WSFrame{Opcode: protocol.OpcodePing})
	}
}
