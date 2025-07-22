// File: internal/concurrency/ring.go
// Package concurrency implements lock-free ring buffers.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// RingBuffer is a bounded circular buffer with atomic head/tail,
// padded to prevent false sharing.
// Implements api.Ring for cross-package consistency.

package concurrency

import (
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// Ensure compile-time interface compliance.
var _ api.Ring[any] = (*RingBuffer[any])(nil)

// RingBuffer is a lock-free ring buffer.
type RingBuffer[T any] struct {
	data []T
	mask uint64
	head uint64
	_    [64]byte // Padding for hot/cold separation
	tail uint64
	_    [64]byte // Padding to separate tail from other data
}

// NewRingBuffer allocates a ring buffer of power-of-two size.
func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size == 0 || size&(size-1) != 0 {
		panic("size must be power of two")
	}
	return &RingBuffer[T]{
		data: make([]T, size),
		mask: size - 1,
	}
}

// Enqueue adds item; returns false if full.
func (r *RingBuffer[T]) Enqueue(item T) bool {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	if tail-head >= uint64(len(r.data)) {
		return false
	}
	r.data[tail&r.mask] = item
	atomic.StoreUint64(&r.tail, tail+1)
	return true
}

// Dequeue removes and returns item; ok false if empty.
func (r *RingBuffer[T]) Dequeue() (T, bool) {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	if head >= tail {
		var zero T
		return zero, false
	}
	item := r.data[head&r.mask]
	atomic.StoreUint64(&r.head, head+1)
	return item, true
}

// Len returns number of items currently in buffer.
func (r *RingBuffer[T]) Len() int {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	return int(tail - head)
}

// Cap returns fixed buffer capacity.
func (r *RingBuffer[T]) Cap() int {
	return len(r.data)
}
