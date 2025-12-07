// File: protocol/connection.go
// Package protocol implements the core WebSocket connection handling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// WSConnection encapsulates a full-duplex WebSocket session.

package protocol

import (
	// "fmt" // DEBUG
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
	
	loopRunning    int32 // Atomic flag
	readBuf        []byte
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
// RecvZeroCopy performs zero-copy receive by consuming the inbox.
// It prioritizes valid flow control by ensuring the inbox is drained by the consumer.
// RecvZeroCopy performs zero-copy receive:
// If RecvLoop is running, it consumes the inbox (Blocking).
// If RecvLoop is NOT running (Server mode), it reads directly from transport.
func (c *WSConnection) RecvZeroCopy() ([]api.Buffer, error) {
	if atomic.LoadInt32(&c.loopRunning) == 1 {
		// Loop Mode: Must consume inbox to prevent deadlock
		select {
		case frame := <-c.inbox:
			// fmt.Println("DEBUG: RecvZeroCopy got frame (inbox)")
			if frame.PayloadLen < 0 || frame.PayloadLen > MaxFramePayload {
				return nil, nil
			}
			buf := c.bufPool.Get(int(frame.PayloadLen), -1)
			payloadBytes := buf.Bytes()
			payloadLen := int(frame.PayloadLen)
			if payloadLen > len(payloadBytes) {
				payloadLen = len(payloadBytes)
			}
			copy(payloadBytes[:payloadLen], frame.Payload)
			// Slice buffer to actual payload size
			slicedBuf := buf.Slice(0, payloadLen)
			// fmt.Printf("DEBUG: RecvZeroCopy Loop Mode returning buf len %d\n", payloadLen)
			return []api.Buffer{slicedBuf}, nil
		case <-c.done:
			return nil, api.ErrTransportClosed
		}
	} else {
		// Direct Mode: Read from transport with Stream Reassembly
		// fmt.Println("DEBUG: RecvZeroCopy Reading Transport")
		raws, err := c.transport.Recv()
		if err != nil {
		// fmt.Printf("DEBUG: Direct Mode Transport Recv Error: %v\n", err)
			return nil, err
		}
		// fmt.Printf("DEBUG: Server Recv got %d buffers\n", len(raws))

		for _, raw := range raws {
			c.readBuf = append(c.readBuf, raw...)
		}

		result := make([]api.Buffer, 0, 4)
		for len(c.readBuf) > 0 {
			// fmt.Printf("DEBUG: Direct Mode Buffer Size: %d\n", len(c.readBuf))
			frame, consumed, err := DecodeFrameFromBytes(c.readBuf)
			if err != nil {
				// fmt.Printf("DEBUG: Direct Decode Error: %v\n", err)
				return nil, err
			}
			if consumed == 0 {
				// fmt.Println("DEBUG: Direct Incomplete")
				break // Incomplete frame
			}
			// fmt.Printf("DEBUG: Direct Decoded Frame Len: %d\n", frame.PayloadLen)

			// Valid frame
			if frame.PayloadLen < 0 || frame.PayloadLen > MaxFramePayload {
				// Fatal error or skip? 
				// DecodeFrameFromBytes handles max payload check.
				// But just in case.
				c.readBuf = c.readBuf[consumed:]
				continue
			}

			buf := c.bufPool.Get(int(frame.PayloadLen), -1)
			payloadBytes := buf.Bytes()
			// Ensure we only use the actual payload length, not the full buffer size
			payloadLen := int(frame.PayloadLen)
			if payloadLen > len(payloadBytes) {
				payloadLen = len(payloadBytes)
			}
			copy(payloadBytes[:payloadLen], frame.Payload)
			
			// Create a sliced buffer that represents only the actual payload
			slicedBuf := buf.Slice(0, payloadLen)

			// Update metrics manually
			atomic.AddInt64(&c.framesReceived, 1)
			atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)
			result = append(result, slicedBuf)

			// Advance buffer
			c.readBuf = c.readBuf[consumed:]
		}
		
		// Optimization: Reset nil if empty to release memory
		if len(c.readBuf) == 0 {
			c.readBuf = nil
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
	atomic.StoreInt32(&c.loopRunning, 1)
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
				// fmt.Printf("DEBUG: recvLoop transport error: %v\n", err)
				// Transport error: terminate connection
				return
			}
			if len(raws) > 0 {
				// fmt.Printf("DEBUG: recvLoop got %d buffers, first len %d\n", len(raws), len(raws[0]))
			}

			for _, raw := range raws {
				c.readBuf = append(c.readBuf, raw...)
			}

		for len(c.readBuf) > 0 {
				frame, consumed, err := DecodeFrameFromBytes(c.readBuf)
				if err != nil {
					// fmt.Printf("DEBUG: Loop Decode Error: %v\n", err)
					return
				}
				if consumed == 0 {
					break // Incomplete
				}
				
				// fmt.Printf("DEBUG: Loop Decoded frame, opcode=%d, payloadLen=%d\n", frame.Opcode, frame.PayloadLen)

				atomic.AddInt64(&c.framesReceived, 1)
				atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)
				
				// CRITICAL: Copy payload before advancing readBuf, as frame.Payload 
				// is a slice into readBuf which will be overwritten on next iteration
				payloadCopy := make([]byte, len(frame.Payload))
				copy(payloadCopy, frame.Payload)
				frame.Payload = payloadCopy
				
				// Advance buffer immediately
				c.readBuf = c.readBuf[consumed:]

				// Handle WebSocket control frames inlining
				if c.handleControl(frame) {
					continue
				}

				// Enqueue for application processing
				select {
				case c.inbox <- frame:
					// fmt.Println("DEBUG: recvLoop pushed to inbox")
				case <-c.done:
					return
				}

				// Copy for Handler (legacy path?)
				// Logic: If handler exists, invoke it?
				// But we rely on inbox consumption.
				// The original code checked handle + invoked.
				// WE MUST KEEP ORIGINAL HANDLER LOGIC?
				// Wait. Original logic: Check Handler -> Invoke.
				// Logic inside `recvLoop` (Step 987):
				/*
					c.mu.RLock()
					h := c.handler
					c.mu.RUnlock()
					if h != nil ... { ... copy & invoke ... }
				*/
				// But we are using RecvZeroCopy (Client) which reads inbox.
				// Does Client use Handler? NO.
				// Client uses RecvZeroCopy.
				// Server uses RecvZeroCopy (Direct).
				// So `recvLoop` Handler invocation is strictly for legacy/async handler registration?
				// If Handler registered, RecvZeroCopy might steal data?
				// `recvLoop` pushes to `inbox`.
				// If `inbox` is consumed by `RecvZeroCopy`.
				// THEN Handler is NOT invoked?
				// Original code: Pushed to `recvQueue` OR `inbox`.
				// Handler invocation logic was separate?
				// No, step 987 logic:
				// 1. Inbox Push.
				// 2. Handler Invoke.
				// BOTH.
				// So frame goes to BOTH?
				// Yes.
				// I should preserve this behavior.
				
				c.mu.RLock()
				h := c.handler
				c.mu.RUnlock()

				if h != nil && frame.PayloadLen <= MaxFramePayload && frame.PayloadLen >= 0 {
					buf := c.bufPool.Get(int(frame.PayloadLen), -1)
					payloadBytes := buf.Bytes()
					if len(payloadBytes) < len(frame.Payload) {
						frame.Payload = frame.Payload[:len(payloadBytes)]
					}
					copy(payloadBytes, frame.Payload)

					go func(b api.Buffer) {
						defer b.Release()
						h.Handle(b)
					}(buf)
				}
			}
			
			if len(c.readBuf) == 0 {
				c.readBuf = nil
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
