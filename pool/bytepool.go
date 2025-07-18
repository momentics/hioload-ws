// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package pool

// BytePool provides zero-copy buffer management (thread/NUMA aware).
type BytePool interface {
    Get() []byte
    Put([]byte)
}

// SimpleBytePool is a trivial, non-threadsafe pool for illustration.
type SimpleBytePool struct {
    bufs chan []byte
    size int
}

// NewSimpleBytePool creates a new pool with the given capacity and buffer size.
func NewSimpleBytePool(capacity, size int) *SimpleBytePool {
    bp := &SimpleBytePool{
        bufs: make(chan []byte, capacity),
        size: size,
    }
    for i := 0; i < capacity; i++ {
        bp.bufs <- make([]byte, size)
    }
    return bp
}

func (bp *SimpleBytePool) Get() []byte {
    select {
    case b := <-bp.bufs:
        return b
    default:
        return make([]byte, bp.size)
    }
}

func (bp *SimpleBytePool) Put(b []byte) {
    select {
    case bp.bufs <- b:
    default:
        // Discard if pool is full.
    }
}
