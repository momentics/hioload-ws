package concurrency

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLockFreeQueue_MPMC(t *testing.T) {
	q := NewLockFreeQueue[int](1024)
	producers := 10
	consumers := 10
	itemsPerProducer := 10000

	var wg sync.WaitGroup
	var sentSum int64
	var receivedSum int64

	// Producers
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				val := pid*itemsPerProducer + i + 1
				for !q.Enqueue(val) {
					// busy wait or yield
					runtime.Gosched()
				}
				atomic.AddInt64(&sentSum, int64(val))
			}
		}(p)
	}

	// Consumers
	var receivedCount int64
	totalItems := int64(producers * itemsPerProducer)

	// Use a closer channel to stop consumers if they get stuck?
	// Or simply wait until all items received.

	consumerWg := sync.WaitGroup{}
	for c := 0; c < consumers; c++ {
		consumerWg.Add(1)
		go func() {
			defer consumerWg.Done()
			for {
				if val, ok := q.Dequeue(); ok {
					atomic.AddInt64(&receivedSum, int64(val))
					if atomic.AddInt64(&receivedCount, 1) == totalItems {
						return // All items received (this logic assumes strict unique processing)
					}
				} else {
					if atomic.LoadInt64(&receivedCount) >= totalItems {
						return
					}
					runtime.Gosched()
				}
			}
		}()
	}

	wg.Wait() // Wait for producers

	// Wait for consumers with timeout
	done := make(chan struct{})
	go func() {
		consumerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if sentSum != receivedSum {
			t.Errorf("Checksum mismatch: sent %d, received %d", sentSum, receivedSum)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Timeout waiting for consumers. Received %d/%d", atomic.LoadInt64(&receivedCount), totalItems)
	}
}

func TestRingBuffer_MPMC(t *testing.T) {
	rb := NewRingBuffer[int](1024)
	producers := 10
	consumers := 10
	itemsPerProducer := 10000

	var wg sync.WaitGroup
	var sentSum int64
	var receivedSum int64
	var receivedCount int64
	totalItems := int64(producers * itemsPerProducer)

	// Producers
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				val := pid*itemsPerProducer + i + 1
				for !rb.Enqueue(val) {
					runtime.Gosched()
				}
				atomic.AddInt64(&sentSum, int64(val))
			}
		}(p)
	}

	// Consumers
	consumerWg := sync.WaitGroup{}
	for c := 0; c < consumers; c++ {
		consumerWg.Add(1)
		go func() {
			defer consumerWg.Done()
			for {
				if val, ok := rb.Dequeue(); ok {
					atomic.AddInt64(&receivedSum, int64(val))
					if atomic.AddInt64(&receivedCount, 1) == totalItems {
						return
					}
				} else {
					if atomic.LoadInt64(&receivedCount) >= totalItems {
						return
					}
					runtime.Gosched()
				}
			}
		}()
	}

	wg.Wait()

	done := make(chan struct{})
	go func() {
		consumerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if sentSum != receivedSum {
			t.Errorf("Checksum mismatch: sent %d, received %d", sentSum, receivedSum)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Timeout waiting for consumers. Received %d/%d", atomic.LoadInt64(&receivedCount), totalItems)
	}
}
