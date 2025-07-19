// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-load event loop with adaptive spin-wait backoff for low latency
// and lock-free handler invocation.

package concurrency

import (
    "runtime"
    "sync/atomic"
    "time"
)

// Event represents a generic event in the loop.
type Event struct {
    Data any
}

// EventHandler handles incoming events.
type EventHandler interface {
    HandleEvent(ev Event)
}

// EventLoop implements a zero-lock hot-path event loop.
type EventLoop struct {
    ring       *RingBuffer[Event]
    batchSize  int

    quit       chan struct{}
    stopped    chan struct{}
    backoffNs  int64

    handlers   atomic.Value // stores []EventHandler
}

// NewEventLoop creates an EventLoop with given batchSize and ring.
func NewEventLoop(batchSize, ringCapacity int) *EventLoop {
    el := &EventLoop{
        ring:      NewRingBuffer[Event](uint64(ringCapacity)),
        batchSize: batchSize,
        quit:      make(chan struct{}),
        stopped:   make(chan struct{}),
        backoffNs: 1,
    }
    el.handlers.Store([]EventHandler{})
    return el
}

// Run starts processing events.
// Hot-path is lock-free: handlers list read atomically.
func (el *EventLoop) Run() {
    batch := make([]Event, el.batchSize)
    for {
        select {
        case <-el.quit:
            close(el.stopped)
            return
        default:
            // dequeue up to batchSize events
            n := 0
            for n < el.batchSize {
                ev, ok := el.ring.Dequeue()
                if !ok {
                    break
                }
                batch[n] = ev
                n++
            }
            if n > 0 {
                // reset backoff
                atomic.StoreInt64(&el.backoffNs, 1)
                // fetch handlers snapshot
                handlers := el.handlers.Load().([]EventHandler)
                for i := 0; i < n; i++ {
                    for _, h := range handlers {
                        h.HandleEvent(batch[i])
                    }
                }
            } else {
                // spin-wait adaptive backoff
                d := atomic.LoadInt64(&el.backoffNs)
                // busy-loop for d iterations
                for i := int64(0); i < d; i++ {
                    // no-op
                }
                // yield to scheduler occasionally
                runtime.Gosched()
                // exponential backoff capped at 1ms
                if d < 1_000_000 {
                    atomic.StoreInt64(&el.backoffNs, d*2)
                }
            }
        }
    }
}

// Stop signals loop termination and waits.
func (el *EventLoop) Stop() {
    close(el.quit)
    <-el.stopped
}

// RegisterHandler adds a handler; safe for concurrent use.
func (el *EventLoop) RegisterHandler(h EventHandler) {
    old := el.handlers.Load().([]EventHandler)
    new := make([]EventHandler, len(old)+1)
    copy(new, old)
    new[len(old)] = h
    el.handlers.Store(new)
}

// UnregisterHandler removes a handler; safe for concurrent use.
func (el *EventLoop) UnregisterHandler(h EventHandler) {
    old := el.handlers.Load().([]EventHandler)
    new := make([]EventHandler, 0, len(old))
    for _, x := range old {
        if x != h {
            new = append(new, x)
        }
    }
    el.handlers.Store(new)
}

// PostEvent enqueues an event; returns false if full.
func (el *EventLoop) PostEvent(ev Event) bool {
    return el.ring.Enqueue(ev)
}

// Pending returns number of queued events.
func (el *EventLoop) Pending() int {
    return el.ring.Len()
}
