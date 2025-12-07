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
	"github.com/momentics/hioload-ws/protocol"
)

// bufEvent wraps an api.Buffer into an api.Event for the reactor.
type bufEvent struct {
	buf api.Buffer
}

// Data returns the underlying buffer payload for event dispatch.
func (e bufEvent) Data() any {
	return e.buf
}

// Ensure bufEvent implements api.Event
var _ api.Event = bufEvent{}

// bufEventWithConn wraps an api.Buffer and a WSConnection for the reactor.
// This allows us to pass connection context (like path) to the handler.
type bufEventWithConn struct {
	buf  api.Buffer
	conn *protocol.WSConnection
}

// Data returns the underlying buffer payload for event dispatch.
func (e bufEventWithConn) Data() any {
	return e.buf
}

// WSConnection returns the WebSocket connection associated with this event.
func (e bufEventWithConn) WSConnection() *protocol.WSConnection {
	return e.conn
}

// Ensure bufEventWithConn implements api.Event
var _ api.Event = bufEventWithConn{}

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

			// Check connection limit before handling the connection
			if s.cfg.MaxConnections > 0 {
				s.connMu.Lock()
				if s.connCount >= int64(s.cfg.MaxConnections) {
					s.connMu.Unlock()
					wsConn.Close() // Close new connection immediately
					continue       // Skip handling this connection
				}
				s.connCount++
				s.connMu.Unlock()
			}

			go s.handleConnWithTracking(wsConn, s.poller)
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

// handleConnWithTracking reads zero-copy buffers from a WSConnection and pushes them into the reactor.
// Also tracks the connection count for limiting.
func (s *Server) handleConnWithTracking(conn *protocol.WSConnection, poller api.Poller) {
	defer func() {
		conn.Close()
		// Decrement connection count when connection is closed
		if s.cfg.MaxConnections > 0 {
			s.connMu.Lock()
			s.connCount--
			s.connMu.Unlock()
		}
	}()

	// Use the connection's inbox channel to receive frames that are processed by recvLoop
	// This avoids competing with recvLoop for transport data
	for {
		select {
		case <-conn.Done(): // Connection closed
			return
		case frame := <-conn.GetInboxChan(): // Get frame from connection's inbox
			if frame != nil {
				// Convert frame payload to buffer for processing
				buf := conn.BufferPool().Get(int(frame.PayloadLen), -1)
				payloadBytes := buf.Bytes()
				if len(payloadBytes) >= len(frame.Payload) {
					copy(payloadBytes, frame.Payload)
					// Push each buffer as a bufEvent into the reactor's inbox.
					// Create an event that contains both the buffer and the connection context
					event := bufEventWithConn{buf: buf, conn: conn}
					poller.Push(event)
				} else {
					buf.Release() // Release if payload is too large for buffer
				}
			}
		}
	}
}


// Shutdown signals Run to stop accepting and processing.
func (s *Server) Shutdown() {
	close(s.shutdownCh)
}
