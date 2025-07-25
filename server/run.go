// File: server/run.go
// Package server implements the core server startup, reactor loop, connection acceptor,
// and graceful shutdown for the hioload-ws `/server` facade.
//: momentics <momentics@gmail.com>
// License: Apache-2.0

package server

import (
	"context"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/protocol"
)

// bufEvent wraps an api.Buffer into a concurrency.Event for the reactor.
type bufEvent struct {
	buf api.Buffer
}

// Data returns the underlying buffer payload for event dispatch.
func (e bufEvent) Data() any {
	return e.buf
}

// Run starts the server: it applies CPU/NUMA affinity, starts the reactor,
// begins accepting WebSocket connections, and blocks until Shutdown() is called.
// It then orchestrates graceful teardown.
func (s *Server) Run(handler api.Handler) error {
	// 1. Pin this OS thread to the configured NUMA node (if any).
	aff := adapters.NewAffinityAdapter()
	if err := aff.Pin(-1, s.cfg.NUMANode); err != nil {
		return err
	}
	defer aff.Unpin()

	// 2. Build middleware-decorated handler chain.
	hChain := NewHandlerChain(handler, s.middleware...)

	// 3. Register the composite handler with the reactor (poller).
	if err := s.poller.Register(hChain); err != nil {
		return err
	}

	// 4. Launch reactor polling loop.
	go func() {
		for {
			select {
			case <-s.shutdownCh:
				return
			default:
				// Poll up to BatchSize events
				s.poller.Poll(s.cfg.BatchSize)
			}
		}
	}()

	// 5. Accept connections and spawn per-connection readers.
	go func() {
		for {
			wsConn, err := s.listener.Accept()
			if err != nil {
				return
			}
			go handleConn(wsConn, s.poller)
		}
	}()

	// 6. Block until Shutdown signal.
	<-s.shutdownCh

	// 7. Graceful teardown.
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()

	s.listener.Close()
	s.poller.Stop()

	// Wait for reactor and readers to finish or timeout.
	<-ctx.Done()
	return nil
}

// handleConn reads zero-copy buffers from a WSConnection and pushes them into the reactor.
func handleConn(conn *protocol.WSConnection, poller api.Poller) {
	defer conn.Close()
	for {
		bufs, err := conn.RecvZeroCopy()
		if err != nil {
			return
		}
		for _, buf := range bufs {
			// Push each buffer as a bufEvent into the reactor’s inbox.
			// Use type assertion to access the Push method.
			poller.(interface{ Push(concurrency.Event) }).Push(bufEvent{buf: buf})
		}
	}
}

// Shutdown signals Run to stop accepting and processing.
func (s *Server) Shutdown() {
	close(s.shutdownCh)
}
