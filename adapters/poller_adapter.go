// File: adapters/poller_adapter.go
// Package adapters
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// PollerAdapter wraps EventLoop for batched event processing.

package adapters

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// PollerAdapter uses EventLoop for batched event processing.
type PollerAdapter struct {
	eventLoop *concurrency.EventLoop
	mu        sync.Mutex
	started   bool
	// handlers and bridges are kept in parallel slices for unregister.
	handlers []api.Handler
	bridges  []*handlerBridge
}

// NewPollerAdapter creates adapter with given parameters.
func NewPollerAdapter(batchSize, ringCapacity int) api.Poller {
	return &PollerAdapter{
		eventLoop: concurrency.NewEventLoop(batchSize, ringCapacity),
		handlers:  make([]api.Handler, 0),
		bridges:   make([]*handlerBridge, 0),
	}
}

type handlerBridge struct {
	inner api.Handler
}

// HandleEvent dispatches to the wrapped api.Handler.
func (hb *handlerBridge) HandleEvent(ev concurrency.Event) {
	hb.inner.Handle(ev.Data)
}

func (p *PollerAdapter) Register(h api.Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		go p.eventLoop.Run()
		p.started = true
	}
	// Create and register a new bridge for this handler
	hb := &handlerBridge{inner: h}
	p.eventLoop.RegisterHandler(hb)
	p.handlers = append(p.handlers, h)
	p.bridges = append(p.bridges, hb)
	return nil
}

func (p *PollerAdapter) Poll(maxEvents int) (int, error) {
	// events are pushed via handlers; report pending count
	count := p.eventLoop.Pending()
	return count, nil
}

func (p *PollerAdapter) Unregister(h api.Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Find the handler and corresponding bridge
	for i, registered := range p.handlers {
		if registered == h {
			// Unregister bridge from EventLoop
			p.eventLoop.UnregisterHandler(p.bridges[i])
			// Remove entries from slices
			p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			p.bridges = append(p.bridges[:i], p.bridges[i+1:]...)
			return nil
		}
	}
	return nil
}

func (p *PollerAdapter) Stop() {
	p.eventLoop.Stop()
}

// Push adds an event to the event loop's inbox for processing.
// This method is not part of the api.Poller interface but is used via type assertion.
func (p *PollerAdapter) Push(ev concurrency.Event) bool {
	return p.eventLoop.Push(ev)
}
