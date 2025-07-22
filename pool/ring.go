// File: pool/ring.go
// Package pool provides a thin adapter for the shared RingBuffer.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// Ring is the shared RingBuffer implementation.
type Ring[T any] = concurrency.RingBuffer[T]

// NewRingBuffer constructs a new ring buffer with given power-of-two capacity.
func NewRingBuffer[T any](size uint64) *Ring[T] {
	return concurrency.NewRingBuffer[T](size)
}

// Ensure it satisfies the api.Ring interface.
var _ api.Ring[any] = (*Ring[any])(nil)
