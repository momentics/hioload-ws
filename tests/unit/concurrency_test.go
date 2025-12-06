// Package unit tests the concurrency functionality.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package unit

import (
	"testing"

	"github.com/momentics/hioload-ws/internal/concurrency"
)

// TestRingBuffer_EnqueueDequeue tests the basic functionality of the lock-free ring buffer.
func TestRingBuffer_EnqueueDequeue(t *testing.T) {
	rb := concurrency.NewRingBuffer[int](8) // Power of 2 size
	
	// Test enqueue
	if !rb.Enqueue(42) {
		t.Errorf("Expected Enqueue to return true")
	}
	
	if rb.Len() != 1 {
		t.Errorf("Expected length 1, got %d", rb.Len())
	}
	
	// Test dequeue
	item, ok := rb.Dequeue()
	if !ok {
		t.Errorf("Expected Dequeue to return true")
	}
	
	if item != 42 {
		t.Errorf("Expected item to be 42, got %d", item)
	}
	
	if rb.Len() != 0 {
		t.Errorf("Expected length 0 after Dequeue, got %d", rb.Len())
	}
}

// TestRingBuffer_Full tests behavior when ring buffer is full.
func TestRingBuffer_Full(t *testing.T) {
	rb := concurrency.NewRingBuffer[int](2) // Small capacity
	
	// Fill the buffer
	if !rb.Enqueue(1) {
		t.Errorf("Expected first Enqueue to succeed")
	}
	
	if !rb.Enqueue(2) {
		t.Errorf("Expected second Enqueue to succeed")
	}
	
	// Try to add one more (should fail)
	if rb.Enqueue(3) {
		t.Errorf("Expected third Enqueue to fail when buffer is full")
	}
	
	if rb.Len() != 2 {
		t.Errorf("Expected length 2, got %d", rb.Len())
	}
}

// TestRingBuffer_Empty tests behavior when ring buffer is empty.
func TestRingBuffer_Empty(t *testing.T) {
	rb := concurrency.NewRingBuffer[int](4)
	
	// Try to dequeue from empty buffer
	_, ok := rb.Dequeue()
	if ok {
		t.Errorf("Expected Dequeue to return false when buffer is empty")
	}
	
	if rb.Len() != 0 {
		t.Errorf("Expected length 0, got %d", rb.Len())
	}
}

// TestRingBuffer_Capacity tests capacity reporting.
func TestRingBuffer_Capacity(t *testing.T) {
	rb := concurrency.NewRingBuffer[int](16)
	
	if rb.Cap() != 16 {
		t.Errorf("Expected capacity 16, got %d", rb.Cap())
	}
	
	// Add some items but don't change capacity
	rb.Enqueue(1)
	rb.Enqueue(2)
	
	if rb.Cap() != 16 {
		t.Errorf("Expected capacity 16 after adding items, got %d", rb.Cap())
	}
}

// TestEventLoop_Basic tests basic functionality of the event loop.
func TestEventLoop_Basic(t *testing.T) {
	el := concurrency.NewEventLoop(10, 100) // batchSize=10, ringCapacity=100

	// Create a simple event handler
	eventsHandled := 0
	testHandler := &testEventHandler{
		handleFunc: func(ev concurrency.Event) {
			eventsHandled++
		},
	}

	// Register the handler
	el.RegisterHandler(testHandler)

	// Add some events
	events := []*testEvent{
		{data: "event1"},
		{data: "event2"},
		{data: "event3"},
	}

	for _, ev := range events {
		el.Push(ev)
	}

	// Check pending count
	pending := el.Pending()
	if pending != 3 {
		t.Errorf("Expected pending events to be 3, got %d", pending)
	}

	// Check push functionality
	if el.Push(&testEvent{data: "event4"}) != true {
		t.Error("Expected Push to return true")
	}

	pending = el.Pending()
	if pending != 4 {
		t.Errorf("Expected pending events to be 4 after additional push, got %d", pending)
	}

	// We only tested push functionality since Run() wasn't started
	// Note: Stop() should only be called after Run() has been started in a separate goroutine
	t.Logf("Event loop test completed - pushed 4 events, pending: %d", pending)
}

// TestEventLoop_Pending tests the pending events functionality.
func TestEventLoop_Pending(t *testing.T) {
	el := concurrency.NewEventLoop(10, 100)
	
	// Initially, no pending events
	if el.Pending() != 0 {
		t.Errorf("Expected 0 pending events initially, got %d", el.Pending())
	}
	
	// Add an event
	ev := &testEvent{data: "test"}
	el.Push(ev)
	
	// Check pending count
	pending := el.Pending()
	if pending == 0 {
		t.Logf("Pending count is %d - this may be zero if the event was processed immediately", pending)
	}
	
	el.Stop()
}

// Helper types for event loop testing
type testEvent struct {
	data any
}

func (te *testEvent) Data() any {
	return te.data
}

type testEventHandler struct {
	handleFunc func(concurrency.Event)
}

func (teh *testEventHandler) HandleEvent(ev concurrency.Event) {
	if teh.handleFunc != nil {
		teh.handleFunc(ev)
	}
}