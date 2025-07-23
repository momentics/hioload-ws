// File: protocol/connection.go
// Package protocol implements the core WebSocket connection handling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This module defines WSConnection, which manages the lifecycle of a WebSocket
// connection over an abstract transport. It handles framing, zero-copy buffer
// integration, control frame processing (ping/pong/close), I/O loops, and
// exposes APIs for deadline control and buffer retrieval. Detailed comments
// explain the implementation choices, concurrency model, and integration points.

package protocol

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// WSConnection encapsulates a full-duplex WebSocket session.
// It decouples the transport (socket, IOCP, DPDK, etc.) from the framing logic,
// enabling zero-copy buffer pooling and high-throughput batch I/O.
type WSConnection struct {
	transport api.Transport  // Underlying I/O abstraction: must support Send, Recv, Close.
	bufPool   api.BufferPool // NUMA-aware buffer pool for zero-copy payload management

	inbox  chan *WSFrame // Channel for decoded incoming frames ready to dispatch
	outbox chan *WSFrame // Channel for outbound frames queued for transport

	mu      sync.RWMutex // Guards handler assignment
	handler api.Handler  // Application-level callback for inbound payloads

	done   chan struct{} // Closed to signal full connection teardown
	closed int32         // Atomic flag: 1 if Close() has been called

	// Statistics counters support observability and live metrics.
	bytesReceived  int64
	bytesSent      int64
	framesReceived int64
	framesSent     int64
}

// NewWSConnection constructs a WSConnection with the specified channel capacity.
// transport: provides raw I/O operations (batch Recv/Send).
// bufPool: supplies zero-copy buffers for payloads.
// channelSize: capacity for internal inbox and outbox channels to buffer bursts.
func NewWSConnection(tr api.Transport, pool api.BufferPool, channelSize int) *WSConnection {
	return &WSConnection{
		transport: tr,
		bufPool:   pool,
		inbox:     make(chan *WSFrame, channelSize),
		outbox:    make(chan *WSFrame, channelSize),
		done:      make(chan struct{}),
	}
}

// Transport exposes the underlying api.Transport to allow setting deadlines
// (e.g., SetReadDeadline / SetWriteDeadline) or querying features.
func (c *WSConnection) Transport() api.Transport {
	return c.transport
}

// RecvZeroCopy performs a zero-copy receive: it invokes transport.Recv(),
// decodes raw frame bytes into protocol.WSFrame objects, allocates pooled
// buffers for payloads, and returns a slice of api.Buffer referencing
// those buffers. Errors propagate transport failures.
func (c *WSConnection) RecvZeroCopy() ([]api.Buffer, error) {
	raws, err := c.transport.Recv()
	if err != nil {
		return nil, err
	}

	// Pre-allocate result slice to avoid intermediate allocations.
	result := make([]api.Buffer, 0, len(raws))

	for _, raw := range raws {
		// DecodeFrameFromBytes parses header, enforces size limits,
		// and returns a WSFrame referencing the raw payload slice.
		frame, err := DecodeFrameFromBytes(raw)
		if err != nil {
			// Skip invalid frames without closing connection.
			continue
		}

		// Allocate a buffer from the pool (NUMA node chosen by pool).
		buf := c.bufPool.Get(int(frame.PayloadLen), -1)

		// Copy payload into pooled buffer. The buffer itself is from the pool,
		// so downstream processing can slice without further GC pressure.
		copy(buf.Bytes(), frame.Payload)

		// Track stats
		atomic.AddInt64(&c.framesReceived, 1)
		atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)

		result = append(result, buf)
	}

	return result, nil
}

// SendFrame enqueues a WSFrame for outbound transmission. Frames are sent
// in FIFO order by sendLoop. Returns ErrTransportClosed if the connection is closed.
func (c *WSConnection) SendFrame(frame *WSFrame) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return api.ErrTransportClosed
	}

	select {
	case c.outbox <- frame:
		// Update stats for queued frame
		atomic.AddInt64(&c.framesSent, 1)
		atomic.AddInt64(&c.bytesSent, frame.PayloadLen)
		return nil
	case <-c.done:
		return api.ErrTransportClosed
	default:
		// Outbox full: could apply backpressure or drop strategy here.
		return errors.New("outbox is full")
	}
}

// Start launches the receive and send loops as separate goroutines.
// These loops run until Close() is called or a fatal transport error occurs.
// The user must call Start() once after creating the connection.
func (c *WSConnection) Start() {
	go c.recvLoop()
	go c.sendLoop()
}

// Close initiates a graceful shutdown: signals loops to exit, closes transport,
// and ensures `done` channel is closed only once. Subsequent calls are no-ops.
func (c *WSConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return nil // Already closed
	}
	// Signal loops to exit
	close(c.done)
	// Close underlying transport (will unblock Recv calls)
	return c.transport.Close()
}

// Done returns a channel that is closed when the connection has been closed.
// Users can use this to wait for full teardown.
func (c *WSConnection) Done() <-chan struct{} {
	return c.done
}

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

				// Invoke handler directly if set, dispatching in separate goroutine
				c.mu.RLock()
				h := c.handler
				c.mu.RUnlock()
				if h != nil {
					buf := c.bufPool.Get(int(frame.PayloadLen), -1)
					copy(buf.Bytes(), frame.Payload)
					go func(b api.Buffer) {
						defer b.Release()
						h.Handle(b)
					}(buf)
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
			data, err := EncodeFrameToBytes(frame)
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

// SetHandler registers an api.Handler to process incoming payload buffers.
// The handler is invoked asynchronously for each received frame payload.
func (c *WSConnection) SetHandler(h api.Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
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
