// File: protocol/connection.go
// Package protocol implements WebSocket connection handling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Enhanced with better facade integration and error handling.

package protocol

import (
	"errors"
	"sync"
	"sync/atomic"

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
	closed  int32 // atomic: 1 if closed
	done    chan struct{}

	// Statistics
	bytesReceived  int64
	bytesSent      int64
	framesReceived int64
	framesSent     int64
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
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, errors.New("connection closed")
	}

	select {
	case frame := <-c.inbox:
		// Convert frame payload to buffer
		buf := c.bufPool.Get(int(frame.PayloadLen), 0)
		copy(buf.Bytes(), frame.Payload)
		atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)
		return []api.Buffer{buf}, nil
	case <-c.done:
		return nil, errors.New("connection closed")
	}
}

// Send enqueues frame for outbound.
func (c *WSConnection) Send(f *WSFrame) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("connection closed")
	}

	select {
	case c.outbox <- f:
		atomic.AddInt64(&c.bytesSent, f.PayloadLen)
		atomic.AddInt64(&c.framesSent, 1)
		return nil
	case <-c.done:
		return errors.New("connection closed")
	default:
		return errors.New("outbox full")
	}
}

// Close gracefully shuts down loops and signals Done.
func (c *WSConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}

	// Close channels to signal shutdown
	close(c.done)

	// Close transport
	if err := c.transport.Close(); err != nil {
		// Log error but don't fail the close operation
		return err
	}

	return nil
}

// GetStats returns connection statistics.
func (c *WSConnection) GetStats() map[string]int64 {
	return map[string]int64{
		"bytes_received":  atomic.LoadInt64(&c.bytesReceived),
		"bytes_sent":      atomic.LoadInt64(&c.bytesSent),
		"frames_received": atomic.LoadInt64(&c.framesReceived),
		"frames_sent":     atomic.LoadInt64(&c.framesSent),
	}
}

// recvLoop reads from transport, decodes frames, and dispatches.
func (c *WSConnection) recvLoop() {
	defer c.Close()

	for atomic.LoadInt32(&c.closed) == 0 {
		raws, err := c.transport.Recv()
		if err != nil {
			// Connection error - close gracefully
			return
		}

		for _, raw := range raws {
			frame, err := DecodeFrameFromBytes(raw)
			if err != nil {
				// Invalid frame - skip but don't close connection
				continue
			}

			atomic.AddInt64(&c.framesReceived, 1)

			// Handle control frames
			if c.handleControlFrame(frame) {
				continue
			}

			// Send to inbox for application processing
			select {
			case c.inbox <- frame:
			case <-c.done:
				return
			default:
				// Inbox full - drop frame (could implement backpressure here)
			}

			// Process with handler if set
			c.mu.RLock()
			h := c.handler
			c.mu.RUnlock()

			if h != nil {
				// Create buffer from frame payload
				buf := c.bufPool.Get(int(frame.PayloadLen), 0)
				copy(buf.Bytes(), frame.Payload)

				// Handle in background to avoid blocking recv loop
				go func(buffer api.Buffer) {
					defer buffer.Release()
					h.Handle(buffer.Bytes())
				}(buf)
			}
		}
	}
}

// sendLoop reads frames, encodes, and sends via transport.
func (c *WSConnection) sendLoop() {
	for {
		select {
		case frame := <-c.outbox:
			data, err := EncodeFrameToBytes(frame)
			if err != nil {
				// Send error - close connection
				c.Close()
				return
			}
			if err := c.transport.Send([][]byte{data}); err != nil {
				// Send error - close connection
				c.Close()
				return
			}
		case <-c.done:
			return
		}
	}
}

// handleControlFrame processes WebSocket control frames (ping/pong/close).
func (c *WSConnection) handleControlFrame(frame *WSFrame) bool {
	switch frame.Opcode {
	case OpcodePing:
		// Respond with pong
		pong := &WSFrame{
			IsFinal:    true,
			Opcode:     OpcodePong,
			PayloadLen: frame.PayloadLen,
			Payload:    frame.Payload,
		}
		c.Send(pong)
		return true

	case OpcodePong:
		// Pong received - just acknowledge
		return true

	case OpcodeClose:
		// Close frame received - echo it back and close
		c.Send(frame)
		c.Close()
		return true

	default:
		return false // Not a control frame
	}
}
