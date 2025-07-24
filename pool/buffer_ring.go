// Package pool adapts the internal concurrency ring buffer as api.Ring.
//
// BufferRing[T] is a thin wrapper over concurrency.RingBuffer[T].
// Provides lock-free FIFO for pipeline or pool management.
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// BufferRing[T] implements api.Ring[T] with power-of-two capacity.
type BufferRing[T any] struct {
	*concurrency.RingBuffer[T]
}

// NewRingBuffer creates a new ring of size `cap`, which must be power of two.
func NewRingBuffer[T any](cap uint64) *BufferRing[T] {
	return &BufferRing[T]{RingBuffer: concurrency.NewRingBuffer[T](cap)}
}

// Ensure compile-time compliance.
var _ api.Ring[any] = (*BufferRing[any])(nil)
