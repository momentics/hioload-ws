// File: facade/hioload.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// HioloadWS facade – orchestrates the core subsystems of the hioload-ws framework,
// providing a simple, composable API for maximal throughput and reliability
// in NUMA-aware, batch-IO, lock-free, and hot-reloadable WebSocket deployments.
//
// This version includes UnregisterHandler, GetContextFactory, GetDebugAPI, and
// RegisterReloadHook in accordance with modern extensibility requirements.

package facade

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/internal/session"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// Config exposes all configurable system parameters
// for optimal control in production setups.
type Config struct {
	IOBufferSize      int
	ChannelSize       int
	RingCapacity      int
	BatchSize         int
	UseDPDK           bool
	ListenAddr        string
	NumWorkers        int
	NUMANode          int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	EnableMetrics     bool
	EnableDebug       bool
	CPUAffinity       bool
	HeartbeatInterval time.Duration
	ShutdownTimeout   time.Duration
}

// DefaultConfig provides a baseline configuration for most use cases.
// You can modify returned fields before passing to New.
func DefaultConfig() *Config {
	return &Config{
		IOBufferSize:      64 * 1024,
		ChannelSize:       64,
		RingCapacity:      1024,
		BatchSize:         16,
		UseDPDK:           false,
		ListenAddr:        ":8080",
		NumWorkers:        4,
		NUMANode:          -1,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		EnableMetrics:     true,
		EnableDebug:       true,
		CPUAffinity:       true,
		HeartbeatInterval: 10 * time.Second,
		ShutdownTimeout:   60 * time.Second,
	}
}

// HioloadWS is the main facade struct, providing access to all core
// infrastructure: transport, pooling, event loop, executor, debugging, etc.
// All relevant control methods for handler lifecycle, context, and debug
// are provided as explicit top-level methods for maximum discoverability.
type HioloadWS struct {
	transport      api.Transport
	bufferPool     api.BufferPool
	poller         api.Poller
	executor       *concurrency.Executor
	scheduler      api.Scheduler
	control        api.Control
	affinity       api.Affinity
	sessionMgr     session.SessionManager
	contextFactory api.ContextFactory // Explicitly expose context factory
	debugAPI       api.Debug          // Explicitly expose debug probe API

	config  *Config
	mu      sync.RWMutex
	started bool
}

// New creates and initializes a new HioloadWS facade instance.
// All construction details are encapsulated for one-call setup.
func New(cfg *Config) (*HioloadWS, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	h := &HioloadWS{config: cfg}

	// Control interface: config, metrics, live debug probes, hot reload hooks.
	h.control = adapters.NewControlAdapter()

	// Attempt to extract the debug probe API from control, if available.
	if debugGetter, ok := h.control.(interface{ GetDebug() api.Debug }); ok {
		h.debugAPI = debugGetter.GetDebug()
	}

	// ContextFactory allows for per-request or per-session key-value context.
	h.contextFactory = adapters.NewContextAdapter()
	// Affinity (NUMA/CPU pinning) for minimizing inter-node memory latency.
	h.affinity = adapters.NewAffinityAdapter()
	// NUMA-aware buffer pool for high-throughput and cache coherence.
	mgr := pool.NewBufferPoolManager()
	h.bufferPool = mgr.GetPool(cfg.NUMANode)

	// Create appropriate transport: DPDK if requested, else platform default.
	var tr api.Transport
	var err error
	if cfg.UseDPDK {
		tr, err = transport.NewDPDKTransport(cfg.IOBufferSize)
		if err != nil {
			log.Printf("DPDK init failed: %v, falling back to native", err)
			tr, err = transport.NewTransport(cfg.IOBufferSize)
		}
	} else {
		tr, err = transport.NewTransport(cfg.IOBufferSize)
	}
	if err != nil {
		return nil, fmt.Errorf("transport init error: %w", err)
	}
	h.transport = tr

	// High-concurrency session manager (sharded by hash).
	h.sessionMgr = session.NewSessionManager(16)
	// Executor: work-stealing, NUMA-aware thread pool for all background processing.
	h.executor = concurrency.NewExecutor(cfg.NumWorkers, cfg.NUMANode)
	// Batched event-poller (reactor).
	h.poller = adapters.NewPollerAdapter(cfg.BatchSize, cfg.RingCapacity)
	// High-res scheduler for timeouts, heartbeats, etc.
	h.scheduler = concurrency.NewScheduler()

	// Initial config pushed into control registry for live metrics/debug.
	h.control.SetConfig(map[string]any{
		"listen_addr": cfg.ListenAddr,
		"transport_type": func() string {
			if cfg.UseDPDK {
				return "dpdk"
			}
			return "native"
		}(),
		"read_timeout_ms":  cfg.ReadTimeout.Milliseconds(),
		"write_timeout_ms": cfg.WriteTimeout.Milliseconds(),
	})

	return h, nil
}

// Start applies any NUMA or CPU pinning,
// enables metrics collection (if configured), and marks service as started.
func (h *HioloadWS) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if h.config.CPUAffinity && h.config.NUMANode >= 0 {
		if err := h.affinity.Pin(h.config.NUMANode, -1); err != nil {
			log.Printf("Affinity pin warning: %v", err)
		}
	}
	if h.config.EnableMetrics {
		h.control.SetConfig(map[string]any{"metrics.enabled": true})
	}
	h.started = true
	return nil
}

// Stop cleanly tears down reactors, executor, transport, and releases affinity.
func (h *HioloadWS) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.started {
		return nil
	}
	h.poller.Stop()
	h.transport.Close()
	h.executor.Close()
	h.affinity.Unpin()
	h.started = false
	return nil
}

// Shutdown ensures all background systems stop gracefully.
func (h *HioloadWS) Shutdown() error {
	return h.Stop()
}

// CreateWebSocketConnection binds together all per-connection
// zero-copy and event-driven state required for each WebSocket session.
func (h *HioloadWS) CreateWebSocketConnection() *protocol.WSConnection {
	return protocol.NewWSConnection(h.transport, h.bufferPool, h.config.ChannelSize)
}

// GetControl exposes the hot reload, dynamic config, metrics, and probe registration interface.
func (h *HioloadWS) GetControl() api.Control {
	return h.control
}

// GetBufferPool exposes the NUMA-aware BufferPool
// for direct zero-copy buffer management by advanced consumers.
func (h *HioloadWS) GetBufferPool() api.BufferPool {
	return h.bufferPool
}

// GetContextFactory grants access to the underlying context factory used in the system.
// This is critical for integrating custom context propagation or request/session-scoped data.
func (h *HioloadWS) GetContextFactory() api.ContextFactory {
	return h.contextFactory
}

// GetDebugAPI provides direct access to the Debug interface,
// allowing for dynamic probe registration and live introspection.
// This is crucial for exposing runtime health and diagnostics in production.
func (h *HioloadWS) GetDebugAPI() api.Debug {
	return h.debugAPI
}

// RegisterHandler adds or replaces a message handler in the event loop.
func (h *HioloadWS) RegisterHandler(handler api.Handler) error {
	return h.poller.Register(handler)
}

// UnregisterHandler removes an existing handler from the poller for dynamic endpoint switching.
func (h *HioloadWS) UnregisterHandler(handler api.Handler) error {
	return h.poller.Unregister(handler)
}

// RegisterReloadHook registers a callback to be invoked when config is hot-reloaded
// via Control.SetConfig. Enables dynamic adaptation, state reset, or metric flush.
func (h *HioloadWS) RegisterReloadHook(fn func()) {
	h.control.OnReload(fn)
}

// Submit dispatches a task to the executor for parallel background processing.
func (h *HioloadWS) Submit(task func()) error {
	return h.executor.Submit(task)
}

// GetScheduler exposes the system-wide scheduler for deferred tasks and timeouts.
func (h *HioloadWS) GetScheduler() api.Scheduler {
	return h.scheduler
}

// GetSessionCount returns the current global count of alive sessions for observability.
func (h *HioloadWS) GetSessionCount() int {
	count := 0
	h.sessionMgr.Range(func(_ session.Session) {
		count++
	})
	return count
}
