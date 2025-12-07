// File: internal/concurrency/eventloop.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// EventLoop is a core reactor for batched event processing optimized for
// NUMA-aware, lock-free, zero-copy systems. It supports dynamic handler
// registration/unregistration, adaptive backoff, batching, and graceful stop.
//
// This version avoids using atomic.CompareAndSwap on slices (which panics),
// replacing it with mutex-protected copy-on-write for handler list updates.

package concurrency

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/momentics/hioload-ws/api"
)

type Event = api.Event

type EventHandler interface {
	// HandleEvent processes a single Event.
	HandleEvent(ev Event)
}

// EventLoop implements a batched, lock-free poller with dynamic handler registration.
// It maintains a slice of EventHandlers protected with a mutex for safe concurrent updates.
type EventLoop struct {
	handlers     atomic.Value  // stores []EventHandler slice (atomically swapped)
	handlersMu   sync.Mutex    // protects writes to handlers slice
	inbox        chan Event    // channel of incoming events
	batchSize    int           // max batch size per poll
	ringCapacity int           // size of event buffer
	quitCh       chan struct{} // closed on Stop()
	doneCh       chan struct{} // closed after Run() exits
	running      atomic.Bool   // running state
}

// NewEventLoop creates a new EventLoop with batchSize and ringCapacity parameters.
// batchSize controls maximum number of events handled in one cycle.
// ringCapacity defines the buffered channel capacity for incoming events.
func NewEventLoop(batchSize, ringCapacity int) *EventLoop {
	el := &EventLoop{
		inbox:        make(chan Event, ringCapacity),
		batchSize:    batchSize,
		ringCapacity: ringCapacity,
		quitCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		running:      atomic.Bool{},
	}
	el.handlers.Store([]EventHandler{}) // initialize with empty slice
	return el
}

// RegisterHandler adds a new event handler atomically and safely.
func (el *EventLoop) RegisterHandler(h EventHandler) {
	el.handlersMu.Lock()
	defer el.handlersMu.Unlock()
	oldHandlers := el.handlers.Load().([]EventHandler)
	newHandlers := make([]EventHandler, len(oldHandlers)+1)
	copy(newHandlers, oldHandlers)
	newHandlers[len(oldHandlers)] = h
	el.handlers.Store(newHandlers)
}

// UnregisterHandler removes a handler safely, if present.
func (el *EventLoop) UnregisterHandler(h EventHandler) {
	el.handlersMu.Lock()
	defer el.handlersMu.Unlock()
	oldHandlers := el.handlers.Load().([]EventHandler)
	newHandlers := make([]EventHandler, 0, len(oldHandlers))
	for _, handler := range oldHandlers {
		if handler != h {
			newHandlers = append(newHandlers, handler)
		}
	}
	el.handlers.Store(newHandlers)
}

// Run starts the event loop which batches events and dispatches them to handlers.
// It runs until Stop is called.
func (el *EventLoop) Run() {
	if !el.running.CompareAndSwap(false, true) {
		return // Already running
	}
	defer func() {
		close(el.doneCh)
		el.running.Store(false)
	}()

	batch := make([]Event, 0, el.batchSize)
	backoffNs := int64(1)
	const maxBackoffNs = int64(1_000_000)

	// Create a reusable timer, initially stopped
	timer := time.NewTimer(0)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	for {
		// Clear batch
		batch = batch[:0]

		// Non-blocking drain upto batchSize events
	DrainLoop:
		for i := 0; i < el.batchSize; i++ {
			select {
			case ev := <-el.inbox:
				batch = append(batch, ev)
			default:
				break DrainLoop
			}
		}

		if len(batch) == 0 {
			// Arm the timer
			timer.Reset(time.Duration(backoffNs) * time.Nanosecond)

			select {
			case <-el.quitCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case ev := <-el.inbox:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				batch = append(batch, ev)
				backoffNs = 1
			case <-timer.C:
				backoffNs *= 2
				if backoffNs > maxBackoffNs {
					backoffNs = maxBackoffNs
				}
			}
		} else {
			// Create snapshot of handlers slice
			handlers := el.handlers.Load().([]EventHandler)
			for _, ev := range batch {
				for _, handler := range handlers {
					handler.HandleEvent(ev)
				}
			}
			backoffNs = 1
		}
	}
}

// Pending returns approximate count of buffered events waiting in inbox.
func (el *EventLoop) Pending() int {
	return len(el.inbox)
}

// Push adds an event to the event loop's inbox for processing.
// Non-blocking, returns false if inbox is full.
func (el *EventLoop) Push(ev Event) bool {
	select {
	case el.inbox <- ev:
		return true
	default:
		return false // inbox is full
	}
}

// Stop signals the Run loop to exit and waits for completion.
func (el *EventLoop) Stop() {
	select {
	case <-el.quitCh:
		// already closed
	default:
		close(el.quitCh)
	}

	// Check if the loop was actually running before waiting
	if el.running.Load() {
		<-el.doneCh
	} else {
		// If not running, just make sure quitCh is closed
		// doneCh is only closed in Run(), so if Run() was never called,
		// we shouldn't wait for doneCh to be closed
	}
}
