// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package protocol

import (
    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/transport"
)

// WebSocketConn: Pool-managed, zero-copy.
type WebSocketConn struct {
    netConn *transport.NetConn
    pool    api.BytePool
}

// NewWebSocketConn creates a new WS conn.
func NewWebSocketConn(c *transport.NetConn, pool api.BytePool) *WebSocketConn {
    return &WebSocketConn{
        netConn: c,
        pool:    pool,
    }
}

// ReadFrame: Faked frame read for illustration.
func (w *WebSocketConn) ReadFrame() (*api.WebSocketFrame, error) {
    buf := w.pool.Get()
    n, err := w.netConn.Read(buf)
    if err != nil {
        w.pool.Put(buf)
        return nil, err
    }
    // Real implementation: parse WS frame, handle payload split.
    return &api.WebSocketFrame{
        Header:  nil, // Omitted for simplicity
        Payload: buf[:n],
        PoolRef: w.pool,
    }, nil
}

// WriteFrame: Faked.
func (w *WebSocketConn) WriteFrame(f *api.WebSocketFrame) error {
    defer f.Release()
    _, err := w.netConn.Write(f.Payload)
    return err
}

// Close closes the underlying net connection.
func (w *WebSocketConn) Close() error {
    return w.netConn.Close()
}
