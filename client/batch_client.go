// File: client/batch_client.go
// Package client: zero-copy batch utilities for client-side operations.
// Author: momentics <momentics@gmail.com>
// License: Apache-20
//
// BatchSender accumulates api.Buffer items in a zero-allocation batch
// using pool.BufferBatch under the hood. Designed for high-throughput
// client workloads where generic types are not required on the client side.

package client

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
)

// BatchSender collects api.Buffer items in a pre-allocated batch.
// It is non-generic: specializes on api.Buffer for client zero-copy paths.
type BatchSender struct {
	batch *pool.BufferBatch
}

// NewBatchSender creates a new BatchSender with the specified capacity.
// Uses pool.BufferBatch internally to avoid allocations on Append.
func NewBatchSender(capacity int) *BatchSender {
	return &BatchSender{
		batch: pool.NewBufferBatch(capacity),
	}
}

// Append adds a pooled api.Buffer to the batch without allocations.
func (bs *BatchSender) Append(buf api.Buffer) {
	bs.batch.Append(buf)
}

// ToSlice returns the underlying []api.Buffer for direct use in transport.Send.
// The returned slice refers to the exact pool.BufferBatch slice.
func (bs *BatchSender) ToSlice() []api.Buffer {
	return bs.batch.Underlying()
}

// Reset clears the batch, retaining its allocated capacity.
func (bs *BatchSender) Reset() {
	bs.batch.Reset()
}
