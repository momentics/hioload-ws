// Package adapters
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package adapters

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// PollerAdapter uses integral BatchSize and RingCapacity.
type PollerAdapter struct {
	eventLoop *concurrency.EventLoop
	handlers  []api.Handler
	mu        sync.RWMutex
	running   bool
}

// NewPollerAdapter creates adapter with explicit parameters.
func NewPollerAdapter(batchSize, ringCapacity int) api.Poller {
	return &PollerAdapter{
		eventLoop: concurrency.NewEventLoop(batchSize, ringCapacity),
		handlers:  make([]api.Handler, 0),
	}
}

type handlerBridge struct {
	inner api.Handler
}

func (hb *handlerBridge) HandleEvent(ev concurrency.Event) {
	hb.inner.Handle(ev.Data)
}

func (p *PollerAdapter) Register(h api.Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		go p.eventLoop.Run()
		p.running = true
	}
	hb := &handlerBridge{inner: h}
	p.eventLoop.RegisterHandler(hb)
	p.handlers = append(p.handlers, h)
	return nil
}

func (p *PollerAdapter) Poll(maxEvents int) (int, error) {
	return p.eventLoop.Pending(), nil
}

func (p *PollerAdapter) Unregister(h api.Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	// No internal removal, event loop handles stale handlers.
	return nil
}

// Stop gracefully stops the event loop.
func (p *PollerAdapter) Stop() {
	p.eventLoop.Stop()
}
