// File: facade/hioload.go
// Unified facade layer for hioload-ws library.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file defines the HioloadWS struct, which aggregates all core components
// of the hioload-ws library behind a single facade. It initializes transport,
// buffer pools, session management, executor, poller, scheduler, affinity, and
// control interfaces based on immutable configuration. The facade exposes
// methods to start/stop the system, create WebSocket connections, submit tasks,
// and retrieve runtime services such as Control, BufferPool, Transport, Scheduler,
// and session count.

package facade

import (
	"fmt"
	"log"
	"sync"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/internal/session"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// Config holds parameters immutable per run.
// All fields influence the initialization of internal components and cannot
// be changed at runtime except via the Control interface which triggers hot-reload.
type Config struct {
	IOBufferSize      int    // Size of I/O buffers for transport layer
	ChannelSize       int    // Capacity of per-connection channel buffers
	RingCapacity      int    // Capacity of internal event loop ring buffers
	BatchSize         int    // Number of events per poller batch
	UseDPDK           bool   // Whether to attempt DPDK transport initialization
	TransportType     string // Human-readable transport type identifier
	ListenAddr        string // TCP address for listeners
	NumWorkers        int    // Number of executor worker goroutines
	NUMANode          int    // Preferred NUMA node for buffer pools and executors
	SessionShards     int    // Number of shards for session manager
	EnableMetrics     bool   // Whether to enable metrics collection
	EnableDebug       bool   // Whether to enable debug probes
	CPUAffinity       bool   // Whether to pin threads to CPUs/NUMA nodes
	HeartbeatInterval int64  // Interval for heartbeat tasks, in nanoseconds
	ShutdownTimeout   int64  // Timeout for graceful shutdown, in nanoseconds
}

// DefaultConfig returns default configuration values.
// These sane defaults support typical use cases without extensive tuning.
func DefaultConfig() *Config {
	return &Config{
		IOBufferSize:      64 * 1024, // 64 KiB I/O buffer size
		ChannelSize:       64,        // 64-frame channel per connection
		RingCapacity:      1024,      // 1024 entries in event loop ring
		BatchSize:         16,        // Process 16 events per poll cycle
		UseDPDK:           false,     // Disable DPDK by default
		TransportType:     "tcp",     // Default transport: TCP sockets
		ListenAddr:        ":8080",   // Listen on port 8080
		NumWorkers:        4,         // Four executor workers
		NUMANode:          -1,        // Auto-select NUMA node
		SessionShards:     16,        // 16 shards for session manager
		EnableMetrics:     true,      // Enable built-in metrics
		EnableDebug:       true,      // Enable debug probes
		CPUAffinity:       true,      // Pin threads by default
		HeartbeatInterval: 10 * 1e9,  // 10-second heartbeat
		ShutdownTimeout:   60 * 1e9,  // 60-second graceful shutdown
	}
}

// HioloadWS is the main facade type.
// It implements api.GracefulShutdown to allow unified shutdown logic.
type HioloadWS struct {
	transport  api.Transport  // Underlying transport (native or DPDK)
	bufferPool api.BufferPool // NUMA-aware buffer pool for zero-copy I/O
	bufferMgr  *pool.BufferPoolManager
	poller     api.Poller            // Batched event reactor
	affinity   api.Affinity          // CPU/NUMA pinning manager
	control    api.Control           // Dynamic config and metrics interface
	executor   *concurrency.Executor // NUMA-aware work-stealing executor
	sessionMgr session.SessionManager
	scheduler  api.Scheduler // High-resolution timer scheduler

	config  *Config      // Immutable configuration
	mu      sync.RWMutex // Protects started flag
	started bool         // Indicates whether Start() has been called
}

// Ensure compliance with api.GracefulShutdown.
var _ api.GracefulShutdown = (*HioloadWS)(nil)

// New constructs HioloadWS with the given configuration.
// It initializes all internal subsystems: control, affinity, transport,
// session manager, executor, poller, scheduler, and exposes scheduler
// parameters via the Control interface for hot-reload.
func New(cfg *Config) (*HioloadWS, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	h := &HioloadWS{config: cfg}

	// Initialize control and affinity adapters.
	h.control = adapters.NewControlAdapter()
	h.affinity = adapters.NewAffinityAdapter()

	// Initialize buffer pool manager and get per-NUMA pool.
	h.bufferMgr = pool.NewBufferPoolManager()
	h.bufferPool = h.bufferMgr.GetPool(cfg.NUMANode)

	// Initialize transport: DPDK or native fallback.
	var tr api.Transport
	var err error
	if cfg.UseDPDK {
		tr, err = transport.NewDPDKTransport(cfg.IOBufferSize)
		if err != nil {
			log.Printf("[facade] DPDK init failed: %v, falling back to native transport", err)
			tr, err = transport.NewTransport(cfg.IOBufferSize)
		}
	} else {
		tr, err = transport.NewTransport(cfg.IOBufferSize)
	}
	if err != nil {
		return nil, fmt.Errorf("transport init failure: %w", err)
	}
	h.transport = tr

	// Initialize session manager, executor, poller, and scheduler.
	h.sessionMgr = session.NewSessionManager(cfg.SessionShards)
	h.executor = concurrency.NewExecutor(cfg.NumWorkers, cfg.NUMANode)
	h.poller = adapters.NewPollerAdapter(cfg.BatchSize, cfg.RingCapacity)
	h.scheduler = concurrency.NewScheduler()

	// Expose configuration values via Control for observability and hot-reload.
	h.control.SetConfig(map[string]any{
		"transport_type":     cfg.TransportType,
		"listen_addr":        cfg.ListenAddr,
		"heartbeat_interval": cfg.HeartbeatInterval,
		"shutdown_timeout":   cfg.ShutdownTimeout,
	})

	return h, nil
}

// Start pins threads according to CPUAffinity and enables metrics if configured.
// Subsequent calls to Start() have no effect.
func (h *HioloadWS) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	// Apply CPU/NUMA affinity if enabled.
	if h.config.CPUAffinity && h.config.NUMANode >= 0 {
		if err := h.affinity.Pin(h.config.NUMANode, -1); err != nil {
			log.Printf("[facade] CPU affinity warning: %v", err)
		}
	}
	// Enable metrics in Control if configured.
	if h.config.EnableMetrics {
		h.control.SetConfig(map[string]any{"metrics.enabled": true})
	}
	h.started = true
	return nil
}

// Stop cleans up resources: stops poller, closes transport and executor,
// unpins CPU affinity, and marks the facade as not started. Calling Stop()
// on a non-started facade is a no-op.
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

// Shutdown implements api.GracefulShutdown by delegating to Stop().
func (h *HioloadWS) Shutdown() error {
	return h.Stop()
}

// GetControl returns the Control interface for dynamic config and metrics.
func (h *HioloadWS) GetControl() api.Control {
	return h.control
}

// GetBufferPool returns the NUMA-aware buffer pool.
func (h *HioloadWS) GetBufferPool() api.BufferPool {
	return h.bufferPool
}

// GetTransport returns the underlying Transport for raw I/O operations.
func (h *HioloadWS) GetTransport() api.Transport {
	return h.transport
}

// Submit dispatches a task to the executor pool for asynchronous execution.
func (h *HioloadWS) Submit(task func()) error {
	return h.executor.Submit(task)
}

// RegisterHandler registers an api.Handler with the internal Poller for event-driven I/O.
func (h *HioloadWS) RegisterHandler(handler api.Handler) error {
	return h.poller.Register(handler)
}

// GetScheduler exposes the high-resolution Scheduler for timed tasks.
func (h *HioloadWS) GetScheduler() api.Scheduler {
	return h.scheduler
}

// GetSessionCount returns the total number of active sessions across all shards.
func (h *HioloadWS) GetSessionCount() int {
	count := 0
	h.sessionMgr.Range(func(s session.Session) {
		count++
	})
	return count
}

// CreateWebSocketConnection constructs a new protocol.WSConnection using
// the facade's transport, buffer pool, and configured channel size.
func (h *HioloadWS) CreateWebSocketConnection() *protocol.WSConnection {
	return protocol.NewWSConnection(h.transport, h.bufferPool, h.config.ChannelSize)
}
