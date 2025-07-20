// File: adapters/poller_adapter.go
// Package adapters
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// PollerAdapter for clean API <-> eventloop bridging.
// Uses Config.BatchSize and Config.RingCapacity instead of literals.

package adapters

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

type PollerAdapter struct {
	eventLoop *concurrency.EventLoop
	handlers  []api.Handler
	mu        sync.RWMutex
	running   bool
}

func NewPollerAdapter(_batchSize, _ringCapacity int) api.Poller {
	// Replace parameters with values from DefaultConfig if zero
	cfg := facade.DefaultConfig()
	batch := cfg.BatchSize
	ring := cfg.RingCapacity
	return &PollerAdapter{
		eventLoop: concurrency.NewEventLoop(batch, ring),
		handlers:  make([]api.Handler, 0),
	}
}

// Inner glue struct: adapts api.Handler to EventHandler
type handlerBridge struct{ inner api.Handler }

func (hb *handlerBridge) HandleEvent(ev concurrency.Event) {
	if buf, ok := ev.Data.([]byte); ok {
		_ = hb.inner.Handle(buf)
	} else if b, ok := ev.Data.(api.Buffer); ok {
		_ = hb.inner.Handle(b)
	} else {
		_ = hb.inner.Handle(ev.Data)
	}
}

func (p *PollerAdapter) Poll(maxEvents int) (int, error) {
	if !p.running {
		go p.eventLoop.Run()
		p.running = true
	}
	return p.eventLoop.Pending(), nil
}

func (p *PollerAdapter) Register(h api.Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	hb := &handlerBridge{inner: h}
	p.eventLoop.RegisterHandler(hb)
	p.handlers = append(p.handlers, h)
	return nil
}

func (p *PollerAdapter) Unregister(h api.Handler) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, orig := range p.handlers {
		if orig == h {
			p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			break
		}
	}
	return nil
}

func (p *PollerAdapter) Stop() {
	p.eventLoop.Stop()
}
