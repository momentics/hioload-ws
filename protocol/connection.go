// Package protocol
// Author: momentics <momentics@gmail.com>
//
// WebSocket connection logic with state management, control frame tracking,
// and integration into reactor/transport/pool layers.

package protocol

import (
	"errors"
	"io"
	"sync"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
)

// WSConnection represents a full connection state machine.
type WSConnection struct {
	transport api.Transport
	inbox     chan *WSFrame
	outbox    chan *WSFrame
	bufPool   api.BufferPool
	closed    bool
	mu        sync.Mutex
}

func (c *WSConnection) SetHandler(mw *adapters.MiddlewareHandler) {
	panic("unimplemented")
}

func (c *WSConnection) Recv() (any, any) {
	panic("unimplemented")
}

// NewWSConnection constructs a new connection from transport and buffers.
func NewWSConnection(t api.Transport, pool api.BufferPool) *WSConnection {
	return &WSConnection{
		transport: t,
		inbox:     make(chan *WSFrame, 64),
		outbox:    make(chan *WSFrame, 64),
		bufPool:   pool,
	}
}

// Start launches internal recv loop.
func (c *WSConnection) Start() {
	go c.recvLoop()
}

// Send queues a frame for transmission.
func (c *WSConnection) Send(f *WSFrame) error {
	select {
	case c.outbox <- f:
		return nil
	default:
		return errors.New("outbox full")
	}
}

// recvLoop reads frames and parses them into inbox channel.
func (c *WSConnection) recvLoop() {
	for !c.closed {
		chunks, err := c.transport.Recv()
		if err != nil {
			continue // optionally log or close
		}
		for _, b := range chunks {
			reader := bytesAsReader(b)
			frame, err := DecodeFrame(reader)
			if err != nil {
				continue
			}
			c.inbox <- frame
		}
	}
}

// Close gracefully shuts down connection.
func (c *WSConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	_ = c.transport.Close()
	c.closed = true
	close(c.inbox)
	close(c.outbox)
}

// bytesAsReader provides io.Reader over byte slice without allocation.
func bytesAsReader(b []byte) io.Reader {
	return &readerFromBytes{buf: b}
}

type readerFromBytes struct {
	buf []byte
	pos int
}

func (r *readerFromBytes) Read(p []byte) (int, error) {
	if r.pos >= len(r.buf) {
		return 0, io.EOF
	}
	n := copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}
