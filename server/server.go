// File: server/server.go
// Package server provides a high-performance, cross-platform WebSocket server facade
// built on hioload-ws primitives: zero-copy,-IO, lock-free, NUMA-aware, etc.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package server

import (
	"errors"
	"sync"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/pool"
)

var ErrAlreadyRunning = errors.New("server already running")

// Server is the unified facade encapsulating listener, reactor, executor, control, and buffer pool.
type Server struct {
	cfg        *Config        // server configuration (batch size, NUMA node, timeouts, etc.)
	control    api.Control    // control adapter for hot-reload, debug probes, metrics
	pool       api.BufferPool // zero-copy buffer pool per NUMA node
	listener   *transport.WebSocketListener
	poller     api.Poller
	executor   api.Executor
	middleware []Middleware
	shutdownCh chan struct{}
	connCount  int64          // current number of active connections
	connMu     sync.RWMutex   // mutex to protect connection count
}

// NewServer constructs a Server facade with the given Config and options.
// It initializes the buffer pool, control adapter, listener, reactor (poller), executor, and applies options.
func NewServer(cfg *Config, opts ...ServerOption) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 1. ControlAdapter for dynamic config, metrics, debug probes, hot-reload
	ctrl := adapters.NewControlAdapter()

	// 2. BufferPoolManager: one pool per NUMA node; choose preferred node or auto
	nodeCount := concurrency.NUMANodes()
	bufMgr := pool.NewBufferPoolManager(nodeCount)
	bufPool := bufMgr.GetPool(cfg.IOBufferSize, cfg.NUMANode)

	// 3. WebSocket listener: zero‐copy buffers, per‐connection channels
	wsListener, err := transport.NewWebSocketListener(
		cfg.ListenAddr,
		bufPool,
		cfg.ChannelCapacity,
		transport.WithListenerNUMANode(cfg.NUMANode),
	)
	if err != nil {
		return nil, err
	}

	// 4. PollerAdapter (Reactor): batch IO, lock-free rings
	poller := adapters.NewPollerAdapter(cfg.BatchSize, cfg.ReactorRing)

	// 5. ExecutorAdapter: lock-free task dispatch, NUMA-aware
	executor := adapters.NewExecutorAdapter(cfg.ExecutorWorkers, cfg.NUMANode)

	srv := &Server{
		cfg:        cfg,
		control:    ctrl,
		pool:       bufPool,
		listener:   wsListener,
		poller:     poller,
		executor:   executor,
		shutdownCh: make(chan struct{}),
	}

	// 6. Apply functional options (middleware, affinity, etc.)
	for _, opt := range opts {
		opt(srv)
	}

	return srv, nil
}

func (s *Server) UseMiddleware(mw ...Middleware) {
    s.middleware = append(s.middleware, mw...)
}

// GetControl returns the ControlAdapter instance.
// Through this, users can register debug probes, query metrics, and perform hot-reload.
func (s *Server) GetControl() api.Control {
	return s.control
}

// GetBufferPool returns the zero-copy BufferPool.
// Consumers can allocate and release Buffer objects without allocations,
// maintaining NUMA locality and high throughput.
func (s *Server) GetBufferPool() api.BufferPool {
	return s.pool
}

// GetActiveConnections returns the current number of active connections.
func (s *Server) GetActiveConnections() int64 {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.connCount
}
