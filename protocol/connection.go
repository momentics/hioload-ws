// File: protocol/connection.go
// Package protocol implements WebSocket connection handling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Added missing SetHandler and Done methods; PayloadLen unified to int64.

package protocol

import (
	"errors"
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// WSConnection manages framing, zero-copy buffers, and optional handler.
type WSConnection struct {
	transport api.Transport
	bufPool   api.BufferPool
	inbox     chan *WSFrame
	outbox    chan *WSFrame

	mu      sync.RWMutex
	handler api.Handler
	closed  bool
	done    chan struct{}
}

// NewWSConnection constructs WSConnection with given channel capacity.
func NewWSConnection(tr api.Transport, pool api.BufferPool, channelSize int) *WSConnection {
	return &WSConnection{
		transport: tr,
		bufPool:   pool,
		inbox:     make(chan *WSFrame, channelSize),
		outbox:    make(chan *WSFrame, channelSize),
		done:      make(chan struct{}),
	}
}

// Start launches I/O loops.
func (c *WSConnection) Start() {
	go c.recvLoop()
	go c.sendLoop()
}

// Done returns channel closed upon connection termination.
func (c *WSConnection) Done() <-chan struct{} {
	return c.done
}

// SetHandler registers a Handler to process incoming payloads.
func (c *WSConnection) SetHandler(h api.Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
}

// Recv returns next buffers or error if closed.
func (c *WSConnection) Recv() ([]api.Buffer, error) {
	frame, ok := <-c.inbox
	if !ok {
		return nil, errors.New("connection closed")
	}
	buf := c.bufPool.Get(int(frame.PayloadLen), 0)
	copy(buf.Bytes(), frame.Payload)
	return []api.Buffer{buf}, nil
}

// Send enqueues frame for outbound.
func (c *WSConnection) Send(f *WSFrame) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return errors.New("connection closed")
	}
	select {
	case c.outbox <- f:
		return nil
	default:
		return errors.New("outbox full")
	}
}

// Close gracefully shuts down loops and signals Done.
func (c *WSConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.outbox)
	_ = c.transport.Close()
	close(c.inbox)
	close(c.done)
	return nil
}

// recvLoop reads from transport, decodes frames, and dispatches.
func (c *WSConnection) recvLoop() {
	defer c.Close()
	for {
		raws, err := c.transport.Recv()
		if err != nil {
			return
		}
		for _, raw := range raws {
			frame, err := DecodeFrameFromBytes(raw)
			if err != nil {
				continue
			}
			select {
			case c.inbox <- frame:
			default:
			}
			c.mu.RLock()
			h := c.handler
			c.mu.RUnlock()
			if h != nil {
				buf := c.bufPool.Get(int(frame.PayloadLen), 0)
				copy(buf.Bytes(), frame.Payload)
				h.Handle(buf)
				buf.Release()
			}
		}
	}
}

// sendLoop reads frames, encodes, and sends via transport.
func (c *WSConnection) sendLoop() {
	for frame := range c.outbox {
		data := EncodeFrameToBytes(frame)
		_ = c.transport.Send([][]byte{data})
	}
}
