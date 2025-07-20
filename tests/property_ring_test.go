// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// property_ring_test.go — Property-based tests for Ring buffer.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/pool"
	"math/rand"
	"time"
)

// PropRingInvariants performs randomized operations to check key invariants.
func TestRingPropertyBased(t *testing.T) {
	for seed := int64(0); seed < 10; seed++ {
		rand.Seed(time.Now().UnixNano() + seed)
		ring := pool.NewRingBuffer[int](64)

		size := 0
		for i := 0; i < 5000; i++ {
			op := rand.Intn(2)
			val := rand.Intn(100000)
			switch op {
			case 0: // enqueue
				if ring.Enqueue(val) {
					size++
				}
			case 1: // dequeue
				_, ok := ring.Dequeue()
				if ok {
					size--
				}
			}
			if size != ring.Len() {
				t.Errorf("Invariant failed: expected %d, got %d", size, ring.Len())
			}
			if ring.Len() < 0 || ring.Len() > 64 {
				t.Fatalf("Ring length out of bounds: %d", ring.Len())
			}
		}
	}
}
