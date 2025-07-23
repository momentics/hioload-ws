// File: internal/concurrency/eventloop.go
// Package concurrency implements a high-performance event loop with adaptive backoff.
// Improvements: unregister all handlers on Stop to avoid dead handlers retention.

package concurrency

import (
	"runtime"
	"sync/atomic"
	"time"
)

type Event struct {
	Data interface{}
}

type EventHandler interface {
	HandleEvent(ev Event)
}

type EventLoop struct {
	queue     *RingBuffer[Event]
	handlers  atomic.Value // []EventHandler
	batchSize int
	stopCh    chan struct{}
	running   int32
	stopped   int32
	backoffNs int64
}

// NewEventLoop creates a new EventLoop.
func NewEventLoop(batchSize, queueSize int) *EventLoop {
	if batchSize <= 0 {
		batchSize = 16
	}
	size := nextPowerOfTwo(uint32(queueSize))
	loop := &EventLoop{
		queue:     NewRingBuffer[Event](uint64(size)),
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
		backoffNs: 1,
	}
	loop.handlers.Store([]EventHandler{})
	return loop
}

func (el *EventLoop) Pending() int {
	return el.queue.Len()
}

func (el *EventLoop) RegisterHandler(h EventHandler) {
	for {
		old := el.handlers.Load().([]EventHandler)
		newSlice := append(old, h)
		if el.handlers.CompareAndSwap(old, newSlice) {
			return
		}
	}
}

func (el *EventLoop) UnregisterHandler(h EventHandler) {
	for {
		old := el.handlers.Load().([]EventHandler)
		var newSlice []EventHandler
		for _, hh := range old {
			if hh != h {
				newSlice = append(newSlice, hh)
			}
		}
		if el.handlers.CompareAndSwap(old, newSlice) {
			return
		}
	}
}

func (el *EventLoop) Post(ev Event) bool {
	return el.queue.Enqueue(ev)
}

func (el *EventLoop) Run() {
	if !atomic.CompareAndSwapInt32(&el.running, 0, 1) {
		return
	}
	defer func() {
		atomic.StoreInt32(&el.stopped, 1)
		// clear handlers on stop
		el.handlers.Store([]EventHandler{})
	}()
	batch := make([]Event, el.batchSize)
	for {
		select {
		case <-el.stopCh:
			return
		default:
			processed := el.processBatch(batch)
			if processed == 0 {
				el.adaptiveBackoff()
			} else {
				atomic.StoreInt64(&el.backoffNs, 1)
			}
		}
	}
}

func (el *EventLoop) Stop() {
	if atomic.LoadInt32(&el.running) == 1 {
		close(el.stopCh)
		for atomic.LoadInt32(&el.stopped) == 0 {
			time.Sleep(time.Microsecond)
		}
	}
}

func (el *EventLoop) processBatch(batch []Event) int {
	count := 0
	handlers := el.handlers.Load().([]EventHandler)
	for i := 0; i < el.batchSize; i++ {
		ev, ok := el.queue.Dequeue()
		if !ok {
			break
		}
		batch[i] = ev
		count++
	}
	for i := 0; i < count; i++ {
		for _, h := range handlers {
			h.HandleEvent(batch[i])
		}
	}
	return count
}

func (el *EventLoop) adaptiveBackoff() {
	select {
	case <-el.stopCh:
		return
	default:
	}
	backoff := atomic.LoadInt64(&el.backoffNs)
	if backoff < 1000 {
		time.Sleep(time.Microsecond)
	} else {
		runtime.Gosched()
	}
	next := backoff * 2
	if next > 1_000_000 {
		next = 1_000_000
	}
	atomic.StoreInt64(&el.backoffNs, next)
}

func nextPowerOfTwo(v uint32) uint32 {
	if v == 0 {
		return 1
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}
