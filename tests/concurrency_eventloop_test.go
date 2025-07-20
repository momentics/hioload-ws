// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// concurrency_eventloop_test.go — Unit test for lock-free event loop.
package tests

import (
	"sync"
	"testing"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

type eventCounter struct {
	count int
	mu    sync.Mutex
}

func (ec *eventCounter) HandleEvent(ev concurrency.Event) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.count++
}

// TestEventLoop_Basic posts events and asserts correct handler calls.
func TestEventLoop_Basic(t *testing.T) {
	loop := concurrency.NewEventLoop(8, 64)
	counter := &eventCounter{}
	loop.RegisterHandler(counter)
	go loop.Run()

	// Post 50 events
	for i := 0; i < 50; i++ {
		ok := loop.Post(concurrency.Event{Data: i})
		if !ok {
			t.Errorf("Failed to enqueue event %d", i)
		}
	}

	// Allow processing
	concurrency.Wait(100)

	// Expect exactly 50 processed events
	counter.mu.Lock()
	if counter.count != 50 {
		t.Errorf("Expected 50 events, got %d", counter.count)
	}
	counter.mu.Unlock()
	loop.Stop()
}
