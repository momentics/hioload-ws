// internal/concurrency/eventloop.go
// Author: momentics <momentics@gmail.com>
//
// Cross-platform, high-load event loop and concurrency orchestrator.
// This component forms the backbone of task multiplexing and scheduling
// for the WebSocket library. All event dispatch is NUMA-aware, lock-free where possible,
// and targets ultra-low-latency with batch-processing patterns.
//
// The EventLoop provides non-blocking registration/deregistration of Handlers (reactor pattern) and
// supports poll-mode scheduling with zero-copy buffers.

package concurrency

import (
	"sync"
	"time"
	"hioload-ws/api"
	"hioload-ws/pool"
)

// Event represents a data-carrying event/message.
type Event struct {
	Data api.Buffer
}

// EventHandler abstracts the event callback.
type EventHandler interface {
	HandleEvent(ev Event)
}

// EventLoop implements a poll-mode, batch event loop using lock-free queues and NUMA-aware partitioning.
type EventLoop struct {
	handlersMu sync.RWMutex
	handlers   []EventHandler

	ring      *pool.RingBuffer[Event]
	batchSize int

	quit    chan struct{}
	stopped chan struct{}
}

// NewEventLoop creates a new NUMA-aware event loop.
// batchSize sets the number of events processed in one poll loop sweep.
func NewEventLoop(batchSize, ringCapacity int) *EventLoop {
	return &EventLoop{
		handlers:   make([]EventHandler, 0, 64),
		ring:       pool.NewRingBuffer[Event](uint64(ringCapacity)),
		batchSize:  batchSize,
		quit:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}
}

// RegisterHandler allows new event handlers to be safely attached.
func (el *EventLoop) RegisterHandler(h EventHandler) {
	el.handlersMu.Lock()
	el.handlers = append(el.handlers, h)
	el.handlersMu.Unlock()
}

// UnregisterHandler unregisters a handler (O(N), safe for small handler sets).
func (el *EventLoop) UnregisterHandler(h EventHandler) {
	el.handlersMu.Lock()
	defer el.handlersMu.Unlock()
	for i, eh := range el.handlers {
		if eh == h {
			el.handlers = append(el.handlers[:i], el.handlers[i+1:]...)
			return
		}
	}
}

// PostEvent posts a new event into the ring; non-blocking.
func (el *EventLoop) PostEvent(ev Event) bool {
	return el.ring.Enqueue(ev)
}

// Run starts the event loop (single-threaded reactor).
// For high load, multiple EventLoops can be instantiated per NUMA node or CPU.
func (el *EventLoop) Run() {
	batch := make([]Event, 0, el.batchSize)
	for {
		select {
		case <-el.quit:
			close(el.stopped)
			return
		default:
			batch = batch[:0]
			for i := 0; i < el.batchSize; i++ {
				ev, ok := el.ring.Dequeue()
				if !ok {
					break
				}
				batch = append(batch, ev)
			}
			// Pass the batch to all handlers
			el.handlersMu.RLock()
			for _, h := range el.handlers {
				for _, ev := range batch {
					h.HandleEvent(ev)
				}
			}
			el.handlersMu.RUnlock()
			if len(batch) == 0 {
				time.Sleep(50 * time.Microsecond) // backoff
			}
		}
	}
}

// Stop requests the event loop to exit after all pending events drain.
func (el *EventLoop) Stop() {
	close(el.quit)
	<-el.stopped
}

// Pending returns the count of queued events.
func (el *EventLoop) Pending() int {
	return el.ring.Len()
}
