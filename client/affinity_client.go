// File: client/batch_client.go
// Package client: batch utilities for Frames.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
package client

import "github.com/momentics/hioload-ws/api"

// FrameBatchSender collects WSFrame pointers into zero-copy batches.
type FrameBatchSender struct {
	batch []*api.Buffer
}

// NewFrameBatchSender creates a batch with given capacity.
func NewFrameBatchSender(capacity int) *FrameBatchSender {
	return &FrameBatchSender{batch: make([]*api.Buffer, 0, capacity)}
}

// Append adds a pooled Buffer holding frame payload.
func (bs *FrameBatchSender) Append(buf api.Buffer) {
	bs.batch = append(bs.batch, &buf)
}

// ToPayloads returns [][]byte slices for transport.Send.
func (bs *FrameBatchSender) ToPayloads() [][]byte {
	out := make([][]byte, len(bs.batch))
	for i, ptr := range bs.batch {
		out[i] = (*ptr).Bytes()
	}
	return out
}

// Reset clears the batch retaining capacity.
func (bs *FrameBatchSender) Reset() {
	bs.batch = bs.batch[:0]
}
