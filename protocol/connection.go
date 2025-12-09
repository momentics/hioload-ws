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

	loopRunning int32 // Atomic flag (recv+send loops running)
	sendRunning int32 // Atomic flag (send loop running)
	readBuf     []byte
}

var frameEncodePool = sync.Pool{
	New: func() any { return make([]byte, 0, 64*1024) },
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
			if frame.Buf.Data != nil {
				return []api.Buffer{frame.Buf}, nil
			}
			payload := frame.Payload
			if len(payload) > int(frame.PayloadLen) {
				payload = payload[:frame.PayloadLen]
			}
			buf := c.bufPool.Get(len(payload), -1)
			dst := buf.Bytes()
			if len(dst) > len(payload) {
				dst = dst[:len(payload)]
			}
			copy(dst, payload)
			return []api.Buffer{buf.Slice(0, len(dst))}, nil
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
			frame, consumed, err := DecodeFrameFromBytes(c.readBuf)
			if err != nil {
				return nil, err
			}
			if consumed == 0 {
				break // Incomplete frame
			}

			if frame.PayloadLen < 0 || frame.PayloadLen > MaxFramePayload {
				c.readBuf = c.readBuf[consumed:]
				continue
			}

			atomic.AddInt64(&c.framesReceived, 1)
			atomic.AddInt64(&c.bytesReceived, frame.PayloadLen)

			payload := frame.Payload
			if len(payload) > int(frame.PayloadLen) {
				payload = payload[:frame.PayloadLen]
			}
			buf := c.bufPool.Get(len(payload), -1)
			dst := buf.Bytes()
			if len(dst) > len(payload) {
				dst = dst[:len(payload)]
			}
			copy(dst, payload)
			result = append(result, buf.Slice(0, len(dst)))

			c.readBuf = c.readBuf[consumed:]
		}

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

	// Ensure send loop is running for batching.
	if atomic.LoadInt32(&c.sendRunning) == 0 {
		if atomic.CompareAndSwapInt32(&c.sendRunning, 0, 1) {
			go c.sendLoop()
		}
	}

	// If background loops are running, prefer queueing for batching.
	if atomic.LoadInt32(&c.sendRunning) == 1 {
		select {
		case c.outbox <- frame:
			return nil
		case <-c.done:
			return api.ErrTransportClosed
		}
	}

	// Try to send directly via transport if sendLoop is not running
	// Use masked encoding if this is a client connection (indicated by Masked field)
	scratch := frameEncodePool.Get().([]byte)
	data, err := EncodeFrameToBufferWithMask(frame, frame.Masked, scratch[:0])
	if err != nil {
		frameEncodePool.Put(scratch[:0])
		return err
	}

	// Send directly via transport (bypass outbox channel)
	if sendErr := c.transport.Send([][]byte{data}); sendErr != nil {
		frameEncodePool.Put(data[:0])
		return sendErr
	}
	frameEncodePool.Put(data[:0])

	atomic.AddInt64(&c.framesSent, 1)
	atomic.AddInt64(&c.bytesSent, frame.PayloadLen)
	return nil
}

// Start launches receive and send loops.
func (c *WSConnection) Start() {
	atomic.StoreInt32(&c.loopRunning, 1)
	atomic.StoreInt32(&c.sendRunning, 1)
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

				// Preserve payload slice; caller may wrap in Buffer without extra copies.
				frame.Buf = api.Buffer{Data: frame.Payload}

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
					if frame.Buf.Data != nil {
						frame.Buf.Release()
					}
					return
				}

				// Invoke handler inline to avoid goroutine churn.
				c.mu.RLock()
				h := c.handler
				c.mu.RUnlock()

				if h != nil && frame.PayloadLen <= MaxFramePayload && frame.PayloadLen >= 0 && frame.Buf.Data != nil {
					buf := frame.Buf
					h.Handle(buf)
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
	const maxBatch = 32
	type batchSlice [][]byte
	var slicePool sync.Pool
	slicePool.New = func() any { return make(batchSlice, 0, maxBatch) }
	for {
		select {
		case <-c.done:
			return
		case frame := <-c.outbox:
			frames := []*WSFrame{frame}
			// Drain additional frames to batch send.
			for len(frames) < maxBatch {
				select {
				case f := <-c.outbox:
					frames = append(frames, f)
				default:
					goto encode
				}
			}
		encode:
			out := slicePool.Get().(batchSlice)[:0]
			for _, fr := range frames {
				scratch := frameEncodePool.Get().([]byte)
				data, err := EncodeFrameToBufferWithMask(fr, fr.Masked, scratch[:0])
				if err != nil {
					frameEncodePool.Put(scratch[:0])
					c.Close()
					return
				}
				out = append(out, data)
			}
			if err := c.transport.Send(out); err != nil {
				for _, buf := range out {
					frameEncodePool.Put(buf[:0])
				}
				slicePool.Put(out[:0])
				c.Close()
				return
			}
			for _, buf := range out {
				frameEncodePool.Put(buf[:0])
			}
			slicePool.Put(out[:0])
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
