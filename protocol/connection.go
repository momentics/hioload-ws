// File: protocol/connection.go
// Package protocol implements the core WebSocket connection handling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// WSConnection encapsulates a full-duplex WebSocket session.

package protocol

import (
	"sync"
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// WSConnection encapsulates a full-duplex WebSocket session.
type WSConnection struct {
	transport api.Transport  // Underlying I/O abstraction
	bufPool   api.BufferPool // NUMA-aware buffer pool
	path      string         // Request path for routing

	inbox  chan *WSFrame
	outbox chan *WSFrame

	mu      sync.RWMutex
	handler api.Handler

	done   chan struct{}
	closed int32

	// Internal queue for frames for RecvZeroCopy when recvLoop is running
	recvQueue chan api.Buffer

	bytesReceived  int64
	bytesSent      int64
	framesReceived int64
	framesSent     int64
}

// NewWSConnection constructs a WSConnection with specified channel capacity and path.
func NewWSConnection(tr api.Transport, pool api.BufferPool, channelSize int) *WSConnection {
	return &WSConnection{
		transport: tr,
		bufPool:   pool,
		inbox:     make(chan *WSFrame, channelSize),
		outbox:    make(chan *WSFrame, channelSize),
		done:      make(chan struct{}),
		recvQueue: make(chan api.Buffer, 64), // Queue for RecvZeroCopy
	}
}

// NewWSConnectionWithPath constructs a WSConnection with specified channel capacity and request path.
func NewWSConnectionWithPath(tr api.Transport, pool api.BufferPool, channelSize int, path string) *WSConnection {
	return &WSConnection{
		transport: tr,
		bufPool:   pool,
		path:      path,
		inbox:     make(chan *WSFrame, channelSize),
		outbox:    make(chan *WSFrame, channelSize),
		done:      make(chan struct{}),
		recvQueue: make(chan api.Buffer, 64), // Queue for RecvZeroCopy
	}
}

// Transport provides access to the underlying api.Transport.
// This enables external wrappers to set I/O deadlines or query transport features.
func (c *WSConnection) Transport() api.Transport {
	return c.transport
}

// Path returns the original request path for routing purposes.
func (c *WSConnection) Path() string {
	return c.path
}

// BufferPool returns the buffer pool associated with this connection.
func (c *WSConnection) BufferPool() api.BufferPool {
	return c.bufPool
}

// RecvZeroCopy performs zero-copy receive:
// If recvLoop is running (using recvQueue), reads from internal queue
// Otherwise, reads directly from transport
func (c *WSConnection) RecvZeroCopy() ([]api.Buffer, error) {
	// Read from internal recvQueue if available (when recvLoop is active)
	select {
	case buf := <-c.recvQueue:
		return []api.Buffer{buf}, nil
	case <-c.done:
		return nil, api.ErrTransportClosed
	default:
		// Fallback to direct transport read if recvQueue is empty
		// This is only for cases when recvLoop isn't running
		raws, err := c.transport.Recv()
		if err != nil {
			return nil, err
		}

		result := make([]api.Buffer, 0, len(raws))
		for _, raw := range raws {
			frame, err := DecodeFrameFromBytes(raw)
			if err != nil {
				continue
			}

			// Validate that frame payload length is within reasonable bounds
			if frame.PayloadLen < 0 || frame.PayloadLen > MaxFramePayload {
				continue // Skip invalid frames to prevent resource exhaustion
			}

			buf := c.bufPool.Get(int(frame.PayloadLen), -1)

			// Perform bounds checking before copying
			payloadBytes := buf.Bytes()
			if len(payloadBytes) < len(frame.Payload) {
				// Truncate payload to fit buffer size if necessary
				frame.Payload = frame.Payload[:len(payloadBytes)]
			}
			copy(payloadBytes, frame.Payload)

			atomic.AddInt64(&c.framesReceived, 1)
			atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)
			result = append(result, buf)
		}
		return result, nil
	}
}

// SendFrame enqueues a WSFrame for outbound transmission.
func (c *WSConnection) SendFrame(frame *WSFrame) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return api.ErrTransportClosed
	}

	// Try to send directly via transport if sendLoop is not running
	// Use masked encoding if this is a client connection (indicated by Masked field)
	data, err := EncodeFrameToBytesWithMask(frame, frame.Masked)
	if err != nil {
		return err
	}

	// Send directly via transport (bypass outbox channel)
	if sendErr := c.transport.Send([][]byte{data}); sendErr != nil {
		return sendErr
	}

	atomic.AddInt64(&c.framesSent, 1)
	atomic.AddInt64(&c.bytesSent, frame.PayloadLen)
	return nil
}

// Start launches receive and send loops.
func (c *WSConnection) Start() {
	go c.recvLoop()
	go c.sendLoop()
}

// GetInboxChan returns the inbox channel for receiving incoming frames.
func (c *WSConnection) GetInboxChan() <-chan *WSFrame {
	return c.inbox
}

// Close initiates shutdown: signals loops and closes transport.
func (c *WSConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil
	}
	close(c.done)
	return c.transport.Close()
}

// Done returns channel closed when connection is closed.
func (c *WSConnection) Done() <-chan struct{} {
	return c.done
}

// SetHandler registers an api.Handler to process incoming payload Buffers.
func (c *WSConnection) SetHandler(h api.Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
}

// Internal loops omitted for brevity...

// recvLoop continuously reads raw frames from transport, decodes them,
// handles control frames (ping/pong/close), and dispatches data frames
// into the inbox channel and optional application handler.
//
// It exits when `done` is closed or a receive error occurs.
func (c *WSConnection) recvLoop() {
	defer c.Close()

	for {
		select {
		case <-c.done:
			return
		default:
			raws, err := c.transport.Recv()
			if err != nil {
				// Transport error: terminate connection
				return
			}

			for _, raw := range raws {
				frame, err := DecodeFrameFromBytes(raw)
				if err != nil {
					continue
				}
				atomic.AddInt64(&c.framesReceived, 1)
				atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)

				// Handle WebSocket control frames inlining
				if c.handleControl(frame) {
					continue
				}

				// Enqueue for application processing
				select {
				case c.inbox <- frame:
				case <-c.done:
					return
				}

				// Create buffer for frame payload to make it available for both inbox and recvQueue
				if frame.PayloadLen <= MaxFramePayload && frame.PayloadLen >= 0 {
					buf := c.bufPool.Get(int(frame.PayloadLen), -1)
					// Perform bounds checking before copying
					payloadBytes := buf.Bytes()
					if len(payloadBytes) < len(frame.Payload) {
						// Truncate payload to fit buffer size if necessary
						frame.Payload = frame.Payload[:len(payloadBytes)]
					}
					copy(payloadBytes, frame.Payload)

					// Invoke handler directly if set, dispatching in separate goroutine
					c.mu.RLock()
					h := c.handler
					c.mu.RUnlock()
					if h != nil {
						go func(b api.Buffer) {
							defer b.Release()
							h.Handle(b)
						}(buf)
					}

					// Also send to recvQueue for RecvZeroCopy calls
					select {
					case c.recvQueue <- buf:
					default:
						// Queue full, release the buffer to prevent memory leak
						buf.Release()
					}
				}
			}
		}
	}
}

// sendLoop reads frames from outbox, encodes them to bytes, and calls
// transport.Send. On send errors, it closes the connection.
func (c *WSConnection) sendLoop() {
	for {
		select {
		case <-c.done:
			return
		case frame := <-c.outbox:
			// Use masked encoding if this is a client connection (indicated by Masked field)
			data, err := EncodeFrameToBytesWithMask(frame, frame.Masked)
			if err != nil {
				c.Close()
				return
			}
			if err := c.transport.Send([][]byte{data}); err != nil {
				c.Close()
				return
			}
		}
	}
}

// handleControl processes ping, pong, and close control frames per RFC6455.
// Returns true if the frame was a control frame that has been handled.
func (c *WSConnection) handleControl(frame *WSFrame) bool {
	switch frame.Opcode {
	case OpcodePing:
		// Immediately respond with Pong using same payload
		pong := &WSFrame{
			IsFinal:    true,
			Opcode:     OpcodePong,
			PayloadLen: frame.PayloadLen,
			Payload:    frame.Payload,
		}
		c.SendFrame(pong)
		return true

	case OpcodePong:
		// Pong acknowledged; metrics can track latency here
		return true

	case OpcodeClose:
		// Echo close and shutdown
		c.SendFrame(frame)
		c.Close()
		return true

	default:
		return false
	}
}

// GetStats returns a snapshot of connection statistics for metrics reporting.
func (c *WSConnection) GetStats() map[string]int64 {
	return map[string]int64{
		"bytes_received":  atomic.LoadInt64(&c.bytesReceived),
		"bytes_sent":      atomic.LoadInt64(&c.bytesSent),
		"frames_received": atomic.LoadInt64(&c.framesReceived),
		"frames_sent":     atomic.LoadInt64(&c.framesSent),
	}
}
