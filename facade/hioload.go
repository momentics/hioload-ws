// File: facade/hioload.go
// Package facade
// Author: momentics <momentics@gmail.com>
//
// Unified facade layer providing a single entry point for hioload-ws library.
// All critical reactor, transport and pooling logic is exposed here.

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

// HioloadWS is the main facade type.
type HioloadWS struct {
	transport   api.Transport
	bufferPool  api.BufferPool
	poller      api.Poller
	affinity    api.Affinity
	control     api.Control
	executor    *concurrency.Executor
	sessionMgr  session.SessionManager
	poolManager *pool.BufferPoolManager
	config      *Config
	started     bool
	mu          sync.RWMutex
}

// Config holds initialization parameters.
type Config struct {
	UseDPDK       bool
	DPDKMode      string
	NumWorkers    int
	NUMANode      int
	CPUAffinity   bool
	BufferSize    int
	RingCapacity  int
	BatchSize     int
	TransportType string
	ListenAddr    string
	SessionShards int
	EnableMetrics bool
	EnableDebug   bool
}

// DefaultConfig returns default config.
func DefaultConfig() *Config {
	return &Config{
		NumWorkers:    4,
		NUMANode:      -1,
		CPUAffinity:   true,
		BufferSize:    65536,
		RingCapacity:  1024,
		BatchSize:     16,
		TransportType: "tcp",
		ListenAddr:    ":8080",
		SessionShards: 16,
		EnableMetrics: true,
		EnableDebug:   true,
	}
}

// New constructs a HioloadWS instance.
// DPDK is optional. If not available or init fails, falls back to standard transport.
func New(config *Config) (*HioloadWS, error) {
	if config == nil {
		config = DefaultConfig()
	}
	h := &HioloadWS{config: config}

	// Core manager and control setup.
	h.control = adapters.NewControlAdapter()
	h.affinity = adapters.NewAffinityAdapter()
	h.poolManager = pool.NewBufferPoolManager()
	h.bufferPool = h.poolManager.GetPool(config.NUMANode)

	var transportImpl api.Transport
	var err error

	// DPDK integration block
	if config.UseDPDK {
		transportImpl, err = transport.NewDPDKTransport(config.DPDKMode)
		if err != nil {
			log.Printf("[facade] DPDK unavailable or init failed: %v, fallback to native transport", err)
			transportImpl, err = transport.NewTransport()
			if err != nil {
				return nil, fmt.Errorf("native transport init failed: %w", err)
			}
		}
	} else {
		transportImpl, err = transport.NewTransport()
		if err != nil {
			return nil, fmt.Errorf("native transport init failed: %w", err)
		}
	}
	h.transport = transportImpl

	h.sessionMgr = session.NewSessionManager(config.SessionShards)
	h.executor = concurrency.NewExecutor(config.NumWorkers, config.NUMANode)
	h.poller = adapters.NewPollerAdapter(config.BatchSize, config.RingCapacity)
	h.control.SetConfig(map[string]any{
		"numa_node":      config.NUMANode,
		"num_workers":    config.NumWorkers,
		"buffer_size":    config.BufferSize,
		"ring_capacity":  config.RingCapacity,
		"batch_size":     config.BatchSize,
		"transport_type": config.TransportType,
		"listen_addr":    config.ListenAddr,
	})

	return h, nil
}

func (h *HioloadWS) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return fmt.Errorf("already started")
	}
	if h.config.CPUAffinity {
		if err := h.affinity.Pin(-1, h.config.NUMANode); err != nil {
			return fmt.Errorf("pin affinity: %w", err)
		}
	}
	if h.config.EnableMetrics {
		h.control.SetConfig(map[string]any{"metrics.enabled": true})
	}
	h.started = true
	return nil
}

func (h *HioloadWS) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.started {
		return nil
	}
	if pa, ok := h.poller.(*adapters.PollerAdapter); ok {
		pa.Stop()
	}
	if err := h.transport.Close(); err != nil {
		return fmt.Errorf("close transport: %w", err)
	}
	h.executor.Close()
	if h.config.CPUAffinity {
		_ = h.affinity.Unpin()
	}
	h.started = false
	return nil
}

func (h *HioloadWS) CreateWebSocketConnection() *protocol.WSConnection {
	return protocol.NewWSConnection(h.transport, h.bufferPool)
}

func (h *HioloadWS) RegisterHandler(handler api.Handler) error {
	return h.poller.Register(handler)
}

func (h *HioloadWS) Submit(task func()) error {
	return h.executor.Submit(task)
}

func (h *HioloadWS) GetBufferPool() api.BufferPool {
	return h.bufferPool
}

func (h *HioloadWS) GetBuffer(size int) api.Buffer {
	return h.bufferPool.Get(size, h.config.NUMANode)
}

func (h *HioloadWS) GetTransport() api.Transport {
	return h.transport
}

func (h *HioloadWS) GetControl() api.Control {
	return h.control
}

func (h *HioloadWS) GetPoller() api.Poller {
	return h.poller
}

func (h *HioloadWS) GetAffinity() api.Affinity {
	return h.affinity
}

func (h *HioloadWS) Poll(maxEvents int) (int, error) {
	return h.poller.Poll(maxEvents)
}

// GetSessionCount returns the number of active sessions.
func (h *HioloadWS) GetSessionCount() int {
	count := 0
	h.sessionMgr.Range(func(s session.Session) {
		count++
	})
	return count
}

func (h *HioloadWS) GetStats() map[string]any {
	stats := h.control.Stats()
	bs := h.bufferPool.Stats()
	stats["buffer_pool.total_alloc"] = bs.TotalAlloc
	stats["buffer_pool.in_use"] = bs.InUse
	stats["buffer_pool.numa_stats"] = bs.NUMAStats
	tf := h.transport.Features()
	stats["transport.zero_copy"] = tf.ZeroCopy
	stats["executor.num_workers"] = h.executor.NumWorkers()
	stats["active_sessions"] = h.GetSessionCount()
	return stats
}
