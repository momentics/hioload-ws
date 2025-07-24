// File: facade/hioload.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// HioloadWS facade – orchestrates the core subsystems of the hioload-ws framework.
// Exposes unified API including explicit Shutdown method for graceful teardown.

package server

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

// Config holds all configurable parameters for HioloadWS.
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

// DefaultConfig returns a baseline configuration for HioloadWS.
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
		HeartbeatInterval: 30 * time.Second,
		ShutdownTimeout:   60 * time.Second,
	}
}

// HioloadWS is the central facade exposing all subsystems.
type HioloadWS struct {
	transport      api.Transport
	bufferPool     api.BufferPool
	poller         api.Poller
	executor       *concurrency.Executor
	scheduler      api.Scheduler
	control        api.Control
	affinity       api.Affinity
	sessionMgr     session.SessionManager
	contextFactory api.ContextFactory
	debugAPI       api.Debug

	config  *Config
	mu      sync.RWMutex
	started bool
}

// New creates and initializes a new HioloadWS facade instance.
func New(cfg *Config) (*HioloadWS, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	h := &HioloadWS{config: cfg}

	// Initialize control, debug, context, affinity, pools, transport, etc.
	h.control = adapters.NewControlAdapter()
	if dbgGetter, ok := h.control.(interface{ GetDebug() api.Debug }); ok {
		h.debugAPI = dbgGetter.GetDebug()
	}
	h.contextFactory = adapters.NewContextAdapter()
	h.affinity = adapters.NewAffinityAdapter()
	mgr := pool.NewBufferPoolManager()
	h.bufferPool = mgr.GetPool(cfg.NUMANode)

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

	h.sessionMgr = session.NewSessionManager(16)
	h.executor = concurrency.NewExecutor(cfg.NumWorkers, cfg.NUMANode)
	h.poller = adapters.NewPollerAdapter(cfg.BatchSize, cfg.RingCapacity)
	h.scheduler = concurrency.NewScheduler()

	// Push initial config for metrics/debug probes
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

// Start activates CPU/NUMA pinning and enables metrics.
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

// Stop stops reactors, transport, executor, and unpins affinity.
// It is internal; users should call Shutdown().
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

// Shutdown provides a clear, public method for graceful shutdown.
// It ensures all subsystems are cleanly stopped.
func (h *HioloadWS) Shutdown() error {
	// Optionally, wait up to config.ShutdownTimeout for in-flight work
	done := make(chan error, 1)
	go func() {
		done <- h.Stop()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(h.config.ShutdownTimeout):
		return fmt.Errorf("shutdown timeout after %v", h.config.ShutdownTimeout)
	}
}

// CreateWebSocketConnection constructs a new zero-copy WSConnection.
func (h *HioloadWS) CreateWebSocketConnection() *protocol.WSConnection {
	return protocol.NewWSConnection(h.transport, h.bufferPool, h.config.ChannelSize)
}

// CreateWebSocketListener returns a native WebSocket listener.
func (h *HioloadWS) CreateWebSocketListener(addr string) (*transport.WebSocketListener, error) {
	return transport.NewWebSocketListener(addr, h.bufferPool, h.config.ChannelSize)
}

// GetControl exposes dynamic config, metrics, and debug probes.
func (h *HioloadWS) GetControl() api.Control {
	return h.control
}

// GetBufferPool returns the NUMA-aware buffer pool.
func (h *HioloadWS) GetBufferPool() api.BufferPool {
	return h.bufferPool
}

// GetContextFactory exposes the context factory.
func (h *HioloadWS) GetContextFactory() api.ContextFactory {
	return h.contextFactory
}

// GetDebugAPI returns the debug probe API.
func (h *HioloadWS) GetDebugAPI() api.Debug {
	return h.debugAPI
}

// RegisterHandler registers a handler with the event poller.
func (h *HioloadWS) RegisterHandler(handler api.Handler) error {
	return h.poller.Register(handler)
}

// UnregisterHandler removes a handler from the poller.
func (h *HioloadWS) UnregisterHandler(handler api.Handler) error {
	return h.poller.Unregister(handler)
}

// Submit dispatches a background task to the executor.
func (h *HioloadWS) Submit(task func()) error {
	return h.executor.Submit(task)
}

// GetScheduler returns the scheduler for timed tasks.
func (h *HioloadWS) GetScheduler() api.Scheduler {
	return h.scheduler
}

// GetSessionCount returns the count of active sessions.
func (h *HioloadWS) GetSessionCount() int {
	count := 0
	h.sessionMgr.Range(func(_ session.Session) { count++ })
	return count
}

