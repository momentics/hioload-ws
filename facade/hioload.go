// File: facade/hioload.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Unified facade layer for the hioload-ws library, orchestrating transport,
// buffer pooling, session management, executors, pollers, schedulers, and
// control interfaces. Exposes a simple API to start/stop the system, create
// WebSocket connections with timeouts, and access runtime services.

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

// Config holds immutable parameters for system initialization. These fields
// determine buffer sizes, batch sizes, transport selection, NUMA preferences,
// and default timeouts for reads and writes.
type Config struct {
	IOBufferSize      int           // Size of transport I/O buffers
	ChannelSize       int           // Capacity of WebSocket recv/send channels
	RingCapacity      int           // Capacity of event-loop ring buffers
	BatchSize         int           // Batch size for poller processing
	UseDPDK           bool          // Whether to initialize DPDK transport
	ListenAddr        string        // TCP listen address for WebSocket listener
	NumWorkers        int           // Number of executor worker goroutines
	NUMANode          int           // Preferred NUMA node for buffers and workers
	ReadTimeout       time.Duration // Default read deadline for WebSocket connections
	WriteTimeout      time.Duration // Default write deadline for WebSocket connections
	EnableMetrics     bool          // Toggle runtime metrics collection
	EnableDebug       bool          // Toggle debug probe registration
	CPUAffinity       bool          // Pin threads to NUMA/Cores if true
	HeartbeatInterval time.Duration // Interval for internal heartbeats (unused)
	ShutdownTimeout   time.Duration // Timeout for graceful shutdown (unused)
}

// DefaultConfig returns a Config populated with sane defaults suitable for
// most deployment scenarios.
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

// HioloadWS is the central facade type, fulfilling api.GracefulShutdown.
type HioloadWS struct {
	transport  api.Transport
	bufferPool api.BufferPool
	poller     api.Poller
	executor   *concurrency.Executor
	scheduler  api.Scheduler
	control    api.Control
	affinity   api.Affinity
	sessionMgr session.SessionManager

	config  *Config
	mu      sync.RWMutex
	started bool
}

// New constructs the facade, initializing subsystems based on Config.
// It sets up transport, buffer pools, session manager, executor, poller,
// scheduler, affinity, and control. Exposes key parameters via Control API.
func New(cfg *Config) (*HioloadWS, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	h := &HioloadWS{config: cfg}

	// Initialize control for dynamic config, metrics, and debug
	h.control = adapters.NewControlAdapter()

	// Initialize affinity for CPU/NUMA pinning
	h.affinity = adapters.NewAffinityAdapter()

	// Setup buffer pools per NUMA node
	mgr := pool.NewBufferPoolManager()
	h.bufferPool = mgr.GetPool(cfg.NUMANode)

	// Initialize transport: DPDK or native fallback
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

	// Initialize session manager, executor, poller, scheduler
	h.sessionMgr = session.NewSessionManager(16)
	h.executor = concurrency.NewExecutor(cfg.NumWorkers, cfg.NUMANode)
	h.poller = adapters.NewPollerAdapter(cfg.BatchSize, cfg.RingCapacity)
	h.scheduler = concurrency.NewScheduler()

	// Publish initial config values into the Control store
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

// Start applies CPU affinity if configured, enables metrics, and marks the
// facade as started. Subsequent calls are no-ops.
func (h *HioloadWS) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	// Pin to NUMA node if requested
	if h.config.CPUAffinity && h.config.NUMANode >= 0 {
		if err := h.affinity.Pin(h.config.NUMANode, -1); err != nil {
			log.Printf("Affinity pin warning: %v", err)
		}
	}
	// Enable metrics flag
	if h.config.EnableMetrics {
		h.control.SetConfig(map[string]any{"metrics.enabled": true})
	}
	h.started = true
	return nil
}

// Stop gracefully shuts down poller, transport, executor, and unpins affinity.
// Marks the facade as not started. Safe to call multiple times.
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

// Shutdown implements api.GracefulShutdown by delegating to Stop.
func (h *HioloadWS) Shutdown() error {
	return h.Stop()
}

// CreateWebSocketConnection builds a protocol.WSConnection using the facade's
// transport, buffer pool, and channel sizing, then wraps it for handler usage.
func (h *HioloadWS) CreateWebSocketConnection() *protocol.WSConnection {
	return protocol.NewWSConnection(h.transport, h.bufferPool, h.config.ChannelSize)
}

// GetControl returns the Control interface for dynamic config and metrics.
func (h *HioloadWS) GetControl() api.Control {
	return h.control
}

// GetBufferPool returns the NUMA-aware BufferPool for zero-copy operations.
func (h *HioloadWS) GetBufferPool() api.BufferPool {
	return h.bufferPool
}

// Submit enqueues a task for asynchronous execution in the executor pool.
func (h *HioloadWS) Submit(task func()) error {
	return h.executor.Submit(task)
}

// RegisterHandler registers an api.Handler with the internal poller for event dispatch.
func (h *HioloadWS) RegisterHandler(handler api.Handler) error {
	return h.poller.Register(handler)
}

// GetScheduler exposes the Scheduler for timed callbacks and heartbeats.
func (h *HioloadWS) GetScheduler() api.Scheduler {
	return h.scheduler
}

// GetSessionCount returns the current number of active sessions across all shards.
func (h *HioloadWS) GetSessionCount() int {
	count := 0
	h.sessionMgr.Range(func(_ session.Session) {
		count++
	})
	return count
}
