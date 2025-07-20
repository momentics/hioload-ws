// File: facade/hioload.go
// Unified facade layer for hioload-ws library.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

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
type Config struct {
	IOBufferSize  int
	ChannelSize   int
	RingCapacity  int
	BatchSize     int
	UseDPDK       bool
	TransportType string
	ListenAddr    string
	NumWorkers    int
	NUMANode      int
	SessionShards int
	EnableMetrics bool
	EnableDebug   bool
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		IOBufferSize:  64 * 1024,
		ChannelSize:   64,
		RingCapacity:  1024,
		BatchSize:     16,
		UseDPDK:       false,
		TransportType: "tcp",
		ListenAddr:    ":8080",
		NumWorkers:    4,
		NUMANode:      -1,
		SessionShards: 16,
		EnableMetrics: true,
		EnableDebug:   true,
	}
}

// HioloadWS is the main facade type.
type HioloadWS struct {
	transport  api.Transport
	bufferPool api.BufferPool
	poller     api.Poller
	affinity   api.Affinity
	control    api.Control
	executor   *concurrency.Executor
	sessionMgr session.SessionManager

	config  *Config
	mu      sync.RWMutex
	started bool
}

// New constructs HioloadWS with immutable parameters.
func New(cfg *Config) (*HioloadWS, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	h := &HioloadWS{config: cfg}

	// Control & Affinity adapters
	h.control = adapters.NewControlAdapter()
	h.affinity = adapters.NewAffinityAdapter()

	// Buffer pool
	poolMgr := pool.NewBufferPoolManager()
	h.bufferPool = poolMgr.GetPool(cfg.NUMANode)

	// Transport
	var tr api.Transport
	var err error
	if cfg.UseDPDK {
		tr, err = transport.NewDPDKTransport(cfg.IOBufferSize)
		if err != nil {
			log.Printf("[facade] DPDK init failed: %v, fallback", err)
			tr, err = transport.NewTransport(cfg.IOBufferSize)
		}
	} else {
		tr, err = transport.NewTransport(cfg.IOBufferSize)
	}
	if err != nil {
		return nil, fmt.Errorf("transport init: %w", err)
	}
	h.transport = tr

	// Session manager
	h.sessionMgr = session.NewSessionManager(cfg.SessionShards)

	// Executor
	h.executor = concurrency.NewExecutor(cfg.NumWorkers, cfg.NUMANode)

	// Poller
	h.poller = adapters.NewPollerAdapter(cfg.BatchSize, cfg.RingCapacity)

	// Dynamic config
	h.control.SetConfig(map[string]any{
		"transport_type": cfg.TransportType,
		"listen_addr":    cfg.ListenAddr,
	})

	return h, nil
}

// Start pins threads and enables metrics.
func (h *HioloadWS) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if h.config.NUMANode >= 0 {
		if err := h.affinity.Pin(h.config.NUMANode, -1); err != nil {
			return err
		}
	}
	if h.config.EnableMetrics {
		h.control.SetConfig(map[string]any{"metrics.enabled": true})
	}
	h.started = true
	return nil
}

// Stop cleans up resources.
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

// GetControl returns the Control interface for dynamic config and debug probes.
func (h *HioloadWS) GetControl() api.Control {
	return h.control
}

// RegisterHandler registers a Handler with the internal Poller.
func (h *HioloadWS) RegisterHandler(handler api.Handler) error {
	return h.poller.Register(handler)
}

// GetSessionCount returns the total number of active sessions.
func (h *HioloadWS) GetSessionCount() int {
	count := 0
	h.sessionMgr.Range(func(s session.Session) {
		count++
	})
	return count
}

// CreateWebSocketConnection constructs a new WSConnection.
func (h *HioloadWS) CreateWebSocketConnection() *protocol.WSConnection {
	return protocol.NewWSConnection(h.transport, h.bufferPool, h.config.ChannelSize)
}
