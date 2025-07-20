// File: internal/concurrency/ring.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Lock-free ring buffer implementation with cache-line optimization.

package concurrency

import (
	"sync/atomic"
)

// RingBuffer is a lock-free, bounded circular buffer.
type RingBuffer[T any] struct {
	_    [8]uint64 // Cache line padding
	head uint64
	_    [7]uint64 // Cache line padding
	tail uint64
	_    [7]uint64 // Cache line padding
	mask uint64
	data []T
}

// NewRingBuffer creates a new ring buffer. Size must be a power of 2.
func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size == 0 || (size&(size-1)) != 0 {
		panic("ring buffer size must be a power of 2")
	}

	return &RingBuffer[T]{
		mask: size - 1,
		data: make([]T, size),
	}
}

// Enqueue adds an item to the ring buffer.
func (r *RingBuffer[T]) Enqueue(item T) bool {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)

	// Check if buffer is full
	if tail-head >= uint64(len(r.data)) {
		return false
	}

	// Store item
	r.data[tail&r.mask] = item

	// Update tail
	atomic.StoreUint64(&r.tail, tail+1)
	return true
}

// Dequeue removes and returns an item from the ring buffer.
func (r *RingBuffer[T]) Dequeue() (T, bool) {
	var zero T
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)

	// Check if buffer is empty
	if head >= tail {
		return zero, false
	}

	// Load item
	item := r.data[head&r.mask]

	// Update head
	atomic.StoreUint64(&r.head, head+1)
	return item, true
}

// Len returns the current number of items in the buffer.
func (r *RingBuffer[T]) Len() int {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	return int(tail - head)
}

// Cap returns the maximum capacity of the buffer.
func (r *RingBuffer[T]) Cap() int {
	return len(r.data)
}

// IsFull returns true if the buffer is full.
func (r *RingBuffer[T]) IsFull() bool {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	return tail-head >= uint64(len(r.data))
}

// IsEmpty returns true if the buffer is empty.
func (r *RingBuffer[T]) IsEmpty() bool {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	return head >= tail
}
