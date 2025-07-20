// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// pool_ring_test.go — Thorough tests for custom lock-free rings (NUMA/cross-thread).
package tests

import (
	"runtime"
	"sync"
	"testing"

	"github.com/momentics/hioload-ws/pool"
)

// TestRingBuffer_Correctness checks basic enqueue/dequeue contract.
func TestRingBuffer_Correctness(t *testing.T) {
	r := pool.NewRingBuffer[int](16)
	for i := 0; i < 16; i++ {
		ok := r.Enqueue(i)
		if !ok {
			t.Fatalf("Enqueue failed at %d", i)
		}
	}
	if !r.IsFull() {
		t.Error("Expected buffer full")
	}
	for i := 0; i < 16; i++ {
		val, ok := r.Dequeue()
		if !ok || val != i {
			t.Fatalf("Expected %d, got %d (ok=%v)", i, val, ok)
		}
	}
	if !r.IsEmpty() {
		t.Error("Expected buffer empty after full cycle")
	}
}

// TestRingBuffer_Concurrent exercises ring with multiple producers/consumers.
func TestRingBuffer_Concurrent(t *testing.T) {
	r := pool.NewRingBuffer[int](128)
	const producers, items = 4, 1000
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < items; i++ {
				for !r.Enqueue(base*items + i) {
					runtime.Gosched()
				}
			}
		}(p)
	}
	got := make(map[int]struct{})
	readDone := make(chan struct{})
	go func() {
		count := 0
		for count < producers*items {
			val, ok := r.Dequeue()
			if ok {
				got[val] = struct{}{}
				count++
			}
		}
		close(readDone)
	}()
	wg.Wait()
	<-readDone
	if len(got) != producers*items {
		t.Errorf("Expected %d unique, got %d", producers*items, len(got))
	}
}
