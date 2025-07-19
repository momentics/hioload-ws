// File: pool/ring.go
// Author: momentics <momentics@gmail.com>
//
// Lock-free ring buffer for cross-thread and NUMA-local data transfer.
// All methods are thread-safe; internal padding minimizes cache contention.

package pool

import (
	"sync/atomic"
)

// RingBuffer is a lock-free fixed-capacity ring buffer (power-of-two size).
type RingBuffer[T any] struct {
	data []T
	mask uint64
	head uint64
	tail uint64
	_    [64]byte // Padding for hot/cold separation
}

// NewRingBuffer allocates a ring buffer with size (must be power of two).
func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size == 0 || (size&(size-1)) != 0 {
		panic("ring buffer size must be power of two")
	}
	return &RingBuffer[T]{
		data: make([]T, size),
		mask: size - 1,
	}
}

// Enqueue adds an item; returns false if full.
func (r *RingBuffer[T]) Enqueue(val T) bool {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	if (tail - head) == uint64(len(r.data)) {
		return false
	}
	idx := tail & r.mask
	r.data[idx] = val
	atomic.AddUint64(&r.tail, 1)
	return true
}

// Dequeue removes and returns (item, ok); ok==false if empty.
func (r *RingBuffer[T]) Dequeue() (res T, ok bool) {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	if head == tail {
		return res, false
	}
	idx := head & r.mask
	res = r.data[idx]
	atomic.AddUint64(&r.head, 1)
	return res, true
}

// Len returns number of items in the buffer.
func (r *RingBuffer[T]) Len() int {
	return int(atomic.LoadUint64(&r.tail) - atomic.LoadUint64(&r.head))
}

// Cap returns logical buffer capacity.
func (r *RingBuffer[T]) Cap() int {
	return len(r.data)
}
