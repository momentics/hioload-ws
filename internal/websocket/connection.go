// internal/websocket/connection.go
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

type Connection struct {
	wsConn       *protocol.WSConnection
	bufferPool   api.BufferPool
	handler      api.Handler
	ctx          context.Context
	cancel       context.CancelFunc
	pingInterval time.Duration
	pongTimeout  time.Duration
	mu           sync.RWMutex
	closed       bool
}

func NewConnection(wsConn *protocol.WSConnection, bufferPool api.BufferPool) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &Connection{
		wsConn:       wsConn,
		bufferPool:   bufferPool,
		ctx:          ctx,
		cancel:       cancel,
		pingInterval: 30 * time.Second,
		pongTimeout:  10 * time.Second,
	}
}

func (c *Connection) SetHandler(handler api.Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
}

func (c *Connection) Start() error {
	c.wsConn.Start()
	go c.keepAlive()
	go c.messageLoop()
	return nil
}

// messageLoop reads from transport and dispatches to handler.
func (c *Connection) messageLoop() {
	defer c.Close()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			raw, err := c.wsConn.Recv()
			if err != nil {
				return
			}
			// Expect raw to be []api.Buffer
			bufs, ok := raw.([]api.Buffer)
			if !ok {
				// Unexpected type; abort
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

func (c *Connection) Send(frame *protocol.WSFrame) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return errors.New("connection closed")
	}
	return c.wsConn.Send(frame)
}

func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.cancel()
	c.wsConn.Close() // removed assignment since Close() returns no value
	return nil
}
