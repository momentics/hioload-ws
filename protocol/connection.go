// File: protocol/connection.go
// Package protocol
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// WebSocket connection logic with state management and integrated event processing.
//
// Added SetHandler to attach an api.Handler for incoming frames.

package protocol

import (
	"bytes"
	"errors"
	"sync"

	"github.com/momentics/hioload-ws/api"
)

type WSConnection struct {
	transport api.Transport
	inbox     chan *WSFrame
	outbox    chan *WSFrame
	bufPool   api.BufferPool

	mu      sync.RWMutex
	handler api.Handler
	closed  bool
	done    chan struct{}
}

// Ensure compile-time interface compliance if needed
// var _ api.Handler = (*WSConnection)(nil)

func NewWSConnection(t api.Transport, pool api.BufferPool) *WSConnection {
	return &WSConnection{
		transport: t,
		inbox:     make(chan *WSFrame, 64),
		outbox:    make(chan *WSFrame, 64),
		bufPool:   pool,
		done:      make(chan struct{}),
	}
}

// SetHandler attaches a Handler to process incoming messages.
func (c *WSConnection) SetHandler(h api.Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
}

// Start launches internal recv and send loops.
func (c *WSConnection) Start() {
	go c.recvLoop()
	go c.sendLoop()
}

// Done returns a channel closed when connection is closed.
func (c *WSConnection) Done() <-chan struct{} {
	return c.done
}

// Recv reads the next batch of buffers from the WebSocket.
func (c *WSConnection) Recv() ([]api.Buffer, error) {
	frame, ok := <-c.inbox
	if !ok {
		return nil, errors.New("connection closed")
	}
	buf := c.bufPool.Get(int(frame.PayloadLen), 0)
	copy(buf.Bytes(), frame.Payload)
	return []api.Buffer{buf}, nil
}

// Send enqueues a frame for transmission.
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

// Close gracefully shuts down the connection.
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

func (c *WSConnection) recvLoop() {
	defer c.Close()
	for {
		raws, err := c.transport.Recv()
		if err != nil {
			return
		}
		for _, raw := range raws {
			frame, err := DecodeFrame(bytes.NewReader(raw))
			if err != nil {
				continue
			}
			select {
			case c.inbox <- frame:
			default:
				// drop if inbox full
			}
		}
	}
}

func (c *WSConnection) sendLoop() {
	for f := range c.outbox {
		buf := encodeFrameBuffer(f)
		_ = c.transport.Send([][]byte{buf})
	}
}

// encodeFrameBuffer encodes a WSFrame into a byte slice.
func encodeFrameBuffer(f *WSFrame) []byte {
	dst := make([]byte, f.PayloadLen+14)
	n, _ := EncodeFrame(dst, f.Opcode, f.Payload, false)
	return dst[:n]
}
