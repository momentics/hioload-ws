// File: internal/websocket/connection.go
// Package websocket
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-performance WebSocket connection handler with integrated event processing.

package websocket

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// Connection represents a managed WebSocket connection.
type Connection struct {
	wsConn       *protocol.WSConnection
	bufferPool   api.BufferPool
	handler      api.Handler
	ctx          context.Context
	cancel       context.CancelFunc
	pingInterval time.Duration
	pongTimeout  time.Duration

	mu     sync.RWMutex
	closed bool
	done   chan struct{}
}

// NewConnection creates a new Connection.
func NewConnection(wsConn *protocol.WSConnection, bufferPool api.BufferPool) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		wsConn:       wsConn,
		bufferPool:   bufferPool,
		ctx:          ctx,
		cancel:       cancel,
		pingInterval: 30 * time.Second,
		pongTimeout:  10 * time.Second,
		done:         make(chan struct{}),
	}
}

// SetHandler sets the api.Handler for incoming messages.
func (c *Connection) SetHandler(handler api.Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

// Start begins the receive and keepalive loops.
func (c *Connection) Start() error {
	c.wsConn.Start()
	go c.keepAlive()
	go c.messageLoop()
	return nil
}

// Done returns a channel that's closed when the connection is fully closed.
func (c *Connection) Done() <-chan struct{} {
	return c.done
}

// messageLoop reads frames and dispatches to handler.
func (c *Connection) messageLoop() {
	defer c.Close()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			bufs, err := c.wsConn.Recv() // Recv now returns ([]api.Buffer, error)
			if err != nil {
				return
			}
			for _, buf := range bufs {
				if c.handler != nil {
					_ = c.handler.Handle(buf)
				}
				buf.Release()
			}
		}
	}
}

// keepAlive sends periodic pings to keep the connection alive.
func (c *Connection) keepAlive() {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			_ = c.wsConn.Send(&protocol.WSFrame{
				IsFinal: true,
				Opcode:  protocol.OpcodePing,
				Payload: []byte("ping"),
			})
		}
	}
}

// Send allows sending a single frame on the connection.
func (c *Connection) Send(frame *protocol.WSFrame) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return errors.New("connection closed")
	}
	return c.wsConn.Send(frame)
}

// Close gracefully terminates the connection.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.cancel()
	_ = c.wsConn.Close()
	close(c.done)
	return nil
}
