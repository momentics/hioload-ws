// File: internal/concurrency/lock_free_queue.go
// Package concurrency provides a lock-free queue for executors.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Single-producer/single-consumer ring buffer with minimal atomics to reduce contention.

package concurrency

import "sync/atomic"

// lockFreeQueue is a ring buffer for one producer, one consumer.
type lockFreeQueue[T any] struct {
	mask    uint64
	entries []T
	head    uint64
	tail    uint64
}

// NewLockFreeQueue creates a new queue with capacity rounded to power of two.
func NewLockFreeQueue[T any](capacity int) *lockFreeQueue[T] {
	size := 1
	for size < capacity {
		size <<= 1
	}
	return &lockFreeQueue[T]{mask: uint64(size - 1), entries: make([]T, size)}
}

// Enqueue adds val; returns false if full.
func (q *lockFreeQueue[T]) Enqueue(val T) bool {
	tail := atomic.LoadUint64(&q.tail)
	head := atomic.LoadUint64(&q.head)
	if tail-head >= uint64(len(q.entries)) {
		return false
	}
	q.entries[tail&q.mask] = val
	atomic.StoreUint64(&q.tail, tail+1)
	return true
}

// Dequeue removes and returns an item; ok false if empty.
func (q *lockFreeQueue[T]) Dequeue() (item T, ok bool) {
	head := atomic.LoadUint64(&q.head)
	tail := atomic.LoadUint64(&q.tail)
	if head >= tail {
		return item, false
	}
	item = q.entries[head&q.mask]
	atomic.StoreUint64(&q.head, head+1)
	return item, true
}
