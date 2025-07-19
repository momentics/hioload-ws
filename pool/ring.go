// Package pool
// Author: momentics <momentics@gmail.com>
//
// Lock-free ring buffer for cross-thread, NUMA-local data transfer.
// Uses atomic operations for concurrency, minimizes cache contention.

package pool

import (
	"sync/atomic"
)

// RingBuffer is a lock-free fixed-capacity ring buffer.
type RingBuffer[T any] struct {
	data     []T
	size     uint64
	mask     uint64
	head     uint64
	tail     uint64
	_        [64]byte // Padding to avoid false sharing
}

// NewRingBuffer allocates a ring buffer with size, must be power of two.
func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size == 0 || (size&(size-1)) != 0 {
		panic("size must be power of 2")
	}
	return &RingBuffer[T]{
		data: make([]T, size),
		size: size,
		mask: size - 1,
	}
}

// Enqueue adds an item; returns false if full.
func (r *RingBuffer[T]) Enqueue(val T) bool {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	if (tail-head) == r.size {
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

// Len returns number of items currently in the buffer.
func (r *RingBuffer[T]) Len() int {
	return int(atomic.LoadUint64(&r.tail) - atomic.LoadUint64(&r.head))
}
func (r *RingBuffer[T]) Cap() int {
	return int(r.size)
}
