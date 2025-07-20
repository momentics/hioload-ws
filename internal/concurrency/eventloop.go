// File: internal/concurrency/eventloop.go
// Package concurrency
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-performance event loop with adaptive backoff and lock-free operation.

package concurrency

import (
	"runtime"
	"sync/atomic"
	"time"
)

// Event represents a generic event in the loop.
type Event struct {
	Type string
	Data interface{}
}

// EventHandler processes events.
type EventHandler interface {
	HandleEvent(event Event)
}

// EventLoop provides a high-performance, poll-mode event processing loop.
type EventLoop struct {
	eventQueue *RingBuffer[Event]
	handlers   atomic.Value // []EventHandler
	batchSize  int
	running    int32
	stopped    int32
	stopCh     chan struct{}
	backoffNs  int64
}

// NewEventLoop creates a new event loop.
func NewEventLoop(batchSize, queueSize int) *EventLoop {
	if batchSize <= 0 {
		batchSize = 16
	}
	if queueSize <= 0 || (queueSize&(queueSize-1)) != 0 {
		queueSize = 1024
	}
	loop := &EventLoop{
		eventQueue: NewRingBuffer[Event](uint64(queueSize)),
		batchSize:  batchSize,
		stopCh:     make(chan struct{}),
		backoffNs:  1,
	}
	loop.handlers.Store([]EventHandler{})
	return loop
}

// RegisterHandler adds an event handler.
func (el *EventLoop) RegisterHandler(handler EventHandler) {
	for {
		old := el.handlers.Load().([]EventHandler)
		new := make([]EventHandler, len(old)+1)
		copy(new, old)
		new[len(old)] = handler
		if el.handlers.CompareAndSwap(old, new) {
			break
		}
	}
}

// UnregisterHandler removes an event handler.
func (el *EventLoop) UnregisterHandler(handler EventHandler) {
	for {
		old := el.handlers.Load().([]EventHandler)
		new := make([]EventHandler, 0, len(old))
		for _, h := range old {
			if h != handler {
				new = append(new, h)
			}
		}
		if el.handlers.CompareAndSwap(old, new) {
			break
		}
	}
}

// PostEvent adds an event to the queue.
func (el *EventLoop) PostEvent(event Event) bool {
	return el.eventQueue.Enqueue(event)
}

// Run starts the event loop.
func (el *EventLoop) Run() {
	if !atomic.CompareAndSwapInt32(&el.running, 0, 1) {
		return // Already running
	}
	defer atomic.StoreInt32(&el.stopped, 1)

	batch := make([]Event, el.batchSize)

	for {
		select {
		case <-el.stopCh:
			return
		default:
			processed := el.processBatch(batch)
			if processed > 0 {
				atomic.StoreInt64(&el.backoffNs, 1)
			} else {
				el.adaptiveBackoff()
			}
		}
	}
}

// processBatch processes up to batchSize events.
func (el *EventLoop) processBatch(batch []Event) int {
	processed := 0
	handlers := el.handlers.Load().([]EventHandler)

	for i := 0; i < el.batchSize; i++ {
		if event, ok := el.eventQueue.Dequeue(); ok {
			batch[i] = event
			processed++
		} else {
			break
		}
	}

	for i := 0; i < processed; i++ {
		for _, handler := range handlers {
			handler.HandleEvent(batch[i])
		}
	}
	return processed
}

// adaptiveBackoff implements exponential backoff with cap and stop check.
func (el *EventLoop) adaptiveBackoff() {
	// check for stop
	select {
	case <-el.stopCh:
		return
	default:
	}

	backoff := atomic.LoadInt64(&el.backoffNs)
	if backoff < 1000 {
		// busy-wait short duration
		time.Sleep(time.Microsecond)
	} else {
		// yield scheduler for longer waits
		runtime.Gosched()
	}

	newBackoff := backoff * 2
	if newBackoff > 1_000_000 {
		newBackoff = 1_000_000
	}
	atomic.StoreInt64(&el.backoffNs, newBackoff)
}

// Stop gracefully stops the event loop.
func (el *EventLoop) Stop() {
	if atomic.LoadInt32(&el.running) == 1 {
		close(el.stopCh)
		for atomic.LoadInt32(&el.stopped) == 0 {
			time.Sleep(time.Microsecond * 100)
		}
	}
}

// Pending returns the number of pending events.
func (el *EventLoop) Pending() int {
	return el.eventQueue.Len()
}

// IsRunning returns true if the event loop is running.
func (el *EventLoop) IsRunning() bool {
	return atomic.LoadInt32(&el.running) == 1
}
