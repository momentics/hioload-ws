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
	handlers  []api.Handler
	mu        sync.Mutex
	started   bool
}

// NewPollerAdapter creates adapter with given parameters.
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
	if !p.started {
		go p.eventLoop.Run()
		p.started = true
	}
	hb := &handlerBridge{inner: h}
	p.eventLoop.RegisterHandler(hb)
	p.handlers = append(p.handlers, h)
	return nil
}

func (p *PollerAdapter) Poll(maxEvents int) (int, error) {
	// events are pushed via handlers; report pending count
	count := p.eventLoop.Pending()
	return count, nil
}

func (p *PollerAdapter) Unregister(h api.Handler) error {
	// Unregistration not supported to avoid contention.
	return nil
}

func (p *PollerAdapter) Stop() {
	p.eventLoop.Stop()
}
