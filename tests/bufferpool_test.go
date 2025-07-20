// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// bufferpool_test.go — Unit tests for buffer pool (NUMA-aware).
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/pool"
)

// TestBufferPool_Basic allocates and releases buffers, checking invariants.
func TestBufferPool_Basic(t *testing.T) {
	// Create a default buffer pool for NUMA node -1 (default)
	pool := pool.NewBufferPoolManager().GetPool(-1)

	// Allocate a buffer of size 1024 bytes
	buf := pool.Get(1024, -1)
	if buf == nil {
		t.Fatal("Expected buffer, got nil")
	}
	if len(buf.Bytes()) != 1024 {
		t.Errorf("Buffer length is not 1024: got %d", len(buf.Bytes()))
	}

	// Write some data into the buffer
	copy(buf.Bytes(), []byte("hello"))
	if string(buf.Bytes()[:5]) != "hello" {
		t.Errorf("Buffer content mismatch")
	}

	// Release the buffer
	buf.Release()

	// Reallocate and check reusability
	buf2 := pool.Get(1024, -1)
	if buf2 == nil {
		t.Fatal("Expected buffer on reallocation")
	}
	buf2.Release()
}

// TestBufferPool_LargeAllocation ensures large allocations succeed or fall back.
func TestBufferPool_LargeAllocation(t *testing.T) {
	pool := pool.NewBufferPoolManager().GetPool(-1)
	// Attempt a large buffer allocation (may fallback to std allocation)
	buf := pool.Get(4*1024*1024, -1)
	if buf == nil {
		t.Fatal("Large buffer allocation failed")
	}
	buf.Release()
}
