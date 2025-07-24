// File: client/batch_client_test.go
// Package client_test: unit tests for BatchSender.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package client_test

import (
	"testing"

	"github.com/momentics/hioload-ws/client"
)

func TestBatchSender(t *testing.T) {
	// Create a buffer pool and two buffers
	pool := client.ClientBufferPool(-1)
	b1 := pool.Get(8, -1)
	b2 := pool.Get(16, -1)

	bs := client.NewBatchSender(2)
	bs.Append(b1)
	bs.Append(b2)

	slice := bs.ToSlice()
	if len(slice) != 2 {
		t.Fatalf("expected 2 buffers, got %d", len(slice))
	}
	// Verify same instances are present
	if slice[0] != b1 || slice[1] != b2 {
		t.Error("batch does not contain expected buffers")
	}

	bs.Reset()
	if len(bs.ToSlice()) != 0 {
		t.Error("expected empty batch after Reset")
	}
}
