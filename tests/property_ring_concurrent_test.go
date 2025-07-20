// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// property_ring_concurrent_test.go — Property-based concurrent ring buffer test.
package tests

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/pool"
)

func TestRingBuffer_PropertyConcurrent(t *testing.T) {
	ring := pool.NewRingBuffer[int](32)
	var wg sync.WaitGroup
	const N = 2000

	// Many writers, many readers running in parallel
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rnd := rand.New(rand.NewSource(seed))
			for j := 0; j < N; j++ {
				val := rnd.Int()
				for !ring.Enqueue(val) {
					time.Sleep(time.Microsecond)
				}
			}
		}(time.Now().UnixNano() + int64(i))
	}
	results := make(map[int]struct{})
	mtx := sync.Mutex{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < N; j++ {
				for {
					val, ok := ring.Dequeue()
					if ok {
						mtx.Lock()
						results[val] = struct{}{}
						mtx.Unlock()
						break
					}
					time.Sleep(time.Microsecond)
				}
			}
		}()
	}
	wg.Wait()
	if len(results) != 4*N {
		t.Errorf("Expected %d results, got %d", 4*N, len(results))
	}
}
