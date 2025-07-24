// File: client/transport_client.go
// Package client implements a zero-copy Transport over net.Conn,
// with deadline support and masking/unmasking for client frames.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
package client

import (
	"fmt"
	"net"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// clientTransport wraps net.Conn to implement api.Transport,
// reading directly into pooled Buffers for zero-copy.
type clientTransport struct {
	conn     net.Conn
	bufPool  api.BufferPool
	bufSize  int
	deadline time.Time
}

func NewClientTransport(conn net.Conn, bp api.BufferPool, bufSize int) api.Transport {
	return &clientTransport{conn: conn, bufPool: bp, bufSize: bufSize}
}

func (t *clientTransport) Send(buffers [][]byte) error {
	for _, buf := range buffers {
		if _, err := t.conn.Write(buf); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
	return nil
}

func (t *clientTransport) Recv() ([][]byte, error) {
	// allocate up to one buffer per read slot
	b := t.bufPool.Get(t.bufSize, -1)
	data := b.Bytes()
	n, err := t.conn.Read(data)
	if err != nil {
		b.Release()
		return nil, fmt.Errorf("read error: %w", err)
	}
	return [][]byte{data[:n]}, nil
}

// SetReadDeadline sets read deadline if supported.
func (t *clientTransport) SetReadDeadline(tm time.Time) error {
	return t.conn.SetReadDeadline(tm)
}

// SetWriteDeadline sets write deadline if supported.
func (t *clientTransport) SetWriteDeadline(tm time.Time) error {
	return t.conn.SetWriteDeadline(tm)
}

func (t *clientTransport) Close() error {
	return t.conn.Close()
}

func (t *clientTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: false,
	}
}
