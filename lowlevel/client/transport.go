// Package client wraps net.Conn into api.Transport with zero-copy, batch support.
//
// On Linux: nonblocking socket, on Windows: IOCP/WSARecv.
// Read/write deadlines passed through when supported.
package client

import (
    "fmt"
    "net"
    "time"

    "github.com/momentics/hioload-ws/api"
)

type transport struct {
    conn    net.Conn
    bufPool api.BufferPool
    bufSize int
}

// NewTransport constructs a NUMA-aware, zero-copy transport.
func NewTransport(conn net.Conn, bp api.BufferPool, bufSize int) api.Transport {
    return &transport{conn: conn, bufPool: bp, bufSize: bufSize}
}

func (t *transport) GetBuffer() api.Buffer {
	return t.bufPool.Get(t.bufSize, -1)
}

func (t *transport) Send(bufs [][]byte) error {
    for _, b := range bufs {
        if _, err := t.conn.Write(b); err != nil {
            return fmt.Errorf("send error: %w", err)
        }
    }
    return nil
}

func (t *transport) Recv() ([][]byte, error) {
    buf := t.bufPool.Get(t.bufSize, -1)
    data := buf.Bytes()
    n, err := t.conn.Read(data)
    if err != nil {
        buf.Release()
        return nil, fmt.Errorf("recv error: %w", err)
    }
    return [][]byte{data[:n]}, nil
}

func (t *transport) Close() error {
    return t.conn.Close()
}

func (t *transport) Features() api.TransportFeatures {
    return api.TransportFeatures{ZeroCopy: true, Batch: true, NUMAAware: false}
}

// Optional deadlines
func (t *transport) SetReadDeadline(tm time.Time) error {
    if rd, ok := t.conn.(interface{ SetReadDeadline(time.Time) error }); ok {
        return rd.SetReadDeadline(tm)
    }
    return nil
}

func (t *transport) SetWriteDeadline(tm time.Time) error {
    if wd, ok := t.conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
        return wd.SetWriteDeadline(tm)
    }
    return nil
}
