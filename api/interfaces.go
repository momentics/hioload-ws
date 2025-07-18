// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package api

import (
    "context"
)

// Reactor represents the event loop and connection registration.
type Reactor interface {
    Run(ctx context.Context) error
    Register(conn NetConn) error
}

// NetConn abstracts a network connection.
type NetConn interface {
    Read(buf []byte) (int, error)
    Write(buf []byte) (int, error)
    Close() error
}

// BytePool defines a zero-copy, reusable buffer pool.
type BytePool interface {
    Get() []byte
    Put([]byte)
}

// ObjectPool defines a generic object pool.
type ObjectPool[T any] interface {
    Get() T
    Put(T)
}

// NumaPoolManager manages pools per NUMA node/CPU.
type NumaPoolManager[T any] interface {
    PoolForNode(nodeID int) ObjectPool[T]
    PoolForCPU(cpuID int) ObjectPool[T]
}

// WebSocketConn contract for WS frame I/O.
type WebSocketConn interface {
    ReadFrame() (*WebSocketFrame, error)
    WriteFrame(*WebSocketFrame) error
    Close() error
}

// WebSocketFrame is a single WS frame.
type WebSocketFrame struct {
    Header  []byte // Zero-copy header slice
    Payload []byte // Zero-copy payload slice (from pool)
    PoolRef BytePool // Reference to owning buffer pool
}

// Release returns buffers to the pool.
func (f *WebSocketFrame) Release() {
    if f.PoolRef != nil && f.Payload != nil {
        f.PoolRef.Put(f.Payload)
        f.Payload = nil
    }
}
