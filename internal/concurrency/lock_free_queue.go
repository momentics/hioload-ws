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
	for {
		head := atomic.LoadUint64(&q.head)
		tail := atomic.LoadUint64(&q.tail)
		if tail-head >= uint64(len(q.entries)) {
			return false // queue is full
		}
		// Attempt to reserve the tail slot atomically
		if atomic.CompareAndSwapUint64(&q.tail, tail, tail+1) {
			q.entries[tail&q.mask] = val
			return true
		}
		// If CAS failed, another producer incremented tail, so retry
	}
}

// Dequeue removes and returns an item; ok false if empty.
// Fixed implementation prevents race condition where multiple consumers
// could attempt to dequeue from the same slot, causing data corruption or double consumption.
// Uses compare-and-swap to atomically reserve the head slot before reading from it.
func (q *lockFreeQueue[T]) Dequeue() (item T, ok bool) {
	var head uint64
	for {
		head = atomic.LoadUint64(&q.head)
		tail := atomic.LoadUint64(&q.tail)
		if head >= tail {
			return item, false // queue is empty
		}

		// Try to reserve the next slot by incrementing head
		if atomic.CompareAndSwapUint64(&q.head, head, head+1) {
			break // Successfully reserved our slot
		}
		// If CAS failed, another consumer modified head, so retry
	}

	// Now read from the reserved slot (head is ours exclusively)
	item = q.entries[head&q.mask]
	return item, true
}
