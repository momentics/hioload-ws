// File: pool/ring.go
// Package pool provides a thin adapter over the shared lock-free ring buffer.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file exports a concrete RingBuffer type parameterized by element type.
// It replaces the unsupported generic alias with a proper struct type,
// ensuring compatibility with both Linux and Windows builds and static analysis.

package pool

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// RingBuffer[T] is a concrete, lock-free ring buffer implementing api.Ring[T].
type RingBuffer[T any] struct {
	*concurrency.RingBuffer[T]
}

// NewRingBuffer constructs a new RingBuffer with the specified power-of-two size.
func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	return &RingBuffer[T]{concurrency.NewRingBuffer[T](size)}
}

// Ensure compile-time compliance with the api.Ring[T] interface.
var _ api.Ring[any] = (*RingBuffer[any])(nil)
