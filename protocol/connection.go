// File: protocol/connection.go
// Package protocol
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// WebSocket connection logic.
// Uses Config.ChannelSize instead of hardcoded 64 for channel capacities.

package protocol

import (
	"errors"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/facade"
)

// WSConnection represents a managed WebSocket connection.
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

// NewWSConnection creates a new WSConnection.
// Replaces channel size literal 64 with Config.ChannelSize.
func NewWSConnection(t api.Transport, pool api.BufferPool) *WSConnection {
	cs := facade.DefaultConfig().ChannelSize
	return &WSConnection{
		transport: t,
		inbox:     make(chan *WSFrame, cs), // use ChannelSize
		outbox:    make(chan *WSFrame, cs), // use ChannelSize
		bufPool:   pool,
		done:      make(chan struct{}),
	}
}

// Start launches internal receive and send loops.
func (c *WSConnection) Start() {
	go c.recvLoop()
	go c.sendLoop()
}

// Done returns a channel closed when the connection is fully closed.
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

// Close gracefully terminates the connection.
func (c *WSConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	// close outgoing channel to stop sendLoop
	close(c.outbox)
	// close transport
	_ = c.transport.Close()
	// close inbox to stop messageLoop
	close(c.inbox)
	// signal done
	close(c.done)
	return nil
}

// recvLoop reads raw data from transport, decodes frames, and sends to inbox.
func (c *WSConnection) recvLoop() {
	defer c.Close()
	for {
		raws, err := c.transport.Recv()
		if err != nil {
			if err == api.ErrTransportClosed {
				// transport closed, exit loop
				return
			}
			// other errors, exit
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

// sendLoop reads frames from outbox and sends via transport.
func (c *WSConnection) sendLoop() {
	for {
		f, ok := <-c.outbox
		if !ok {
			// outbox closed, exit sendLoop
			return
		}
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

// SetHandler attaches a Handler to process incoming messages.
func (c *WSConnection) SetHandler(h api.Handler) {
	c.mu.Lock()
	c.handler = h
	c.mu.Unlock()
}
