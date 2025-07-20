// File: internal/concurrency/lock_free_queue.go
// Package concurrency provides a lock-free queue for work-stealing executors.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// lockFreeQueue is a single-producer/single-consumer ring buffer
// with capacity power-of-two. Multiple queues can be used in work-stealing.
//
// Enqueue is safe for one producer; Dequeue is safe for one consumer.
// Stealing from another queue is non-blocking and may fail if empty.

package concurrency

import (
	"sync/atomic"
)

// lockFreeQueue holds tasks in a ring buffer without locks.
type lockFreeQueue[T any] struct {
	mask    uint64   // size-1
	entries []T      // underlying slice
	head    uint64   // consumer index
	tail    uint64   // producer index
	_pad    [48]byte // padding to avoid false sharing
}

// NewLockFreeQueue creates a new queue with given capacity.
// Capacity must be power of two; otherwise it's rounded up.
func NewLockFreeQueue[T any](capacity int) *lockFreeQueue[T] {
	// round up to next power-of-two
	size := 1
	for size < capacity {
		size <<= 1
	}
	return &lockFreeQueue[T]{
		mask:    uint64(size - 1),
		entries: make([]T, size),
	}
}

// Enqueue adds an item; returns false if full.
func (q *lockFreeQueue[T]) Enqueue(val T) bool {
	tail := atomic.LoadUint64(&q.tail)
	head := atomic.LoadUint64(&q.head)
	if tail-head >= uint64(len(q.entries)) {
		return false // full
	}
	q.entries[tail&q.mask] = val
	atomic.StoreUint64(&q.tail, tail+1)
	return true
}

// Dequeue removes and returns an item; ok==false if empty.
func (q *lockFreeQueue[T]) Dequeue() (item T, ok bool) {
	head := atomic.LoadUint64(&q.head)
	tail := atomic.LoadUint64(&q.tail)
	if head >= tail {
		return item, false // empty
	}
	item = q.entries[head&q.mask]
	atomic.StoreUint64(&q.head, head+1)
	return item, true
}

// Len returns current number of items.
func (q *lockFreeQueue[T]) Len() int {
	head := atomic.LoadUint64(&q.head)
	tail := atomic.LoadUint64(&q.tail)
	return int(tail - head)
}

// Cap returns fixed capacity.
func (q *lockFreeQueue[T]) Cap() int {
	return len(q.entries)
}
