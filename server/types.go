// File: server/types.go
// Package server defines high-level Server API and configuration.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package server

import (
	"runtime"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// Config holds all server parameters for high-performance WebSocket service.
type Config struct {
	ListenAddr      string            // ":port"
	IOBufferSize    int               // size of zero-copy buffers
	ChannelCapacity int               // capacity of per-connection frame channels
	NUMANode        int               // preferred NUMA node (-1 = auto)
	ReadTimeout     time.Duration     // optional read deadline
	WriteTimeout    time.Duration     // optional write deadline
	BatchSize       int               // number of events per reactor batch
	ReactorRing     int               // capacity of reactor ring buffer
	ExecutorWorkers int               // number of executor workers
	AffinityScope   api.AffinityScope // CPU/NUMA binding scope
	ShutdownTimeout time.Duration     // graceful shutdown wait time
}

// DefaultConfig returns safe defaults optimized for throughput and latency.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:      ":9000",
		IOBufferSize:    64 * 1024,
		ChannelCapacity: 64,
		NUMANode:        -1,
		ReadTimeout:     0,
		WriteTimeout:    0,
		BatchSize:       32,
		ReactorRing:     1024,
		ExecutorWorkers: runtime.NumCPU(),
		AffinityScope:   api.ScopeThread,
		ShutdownTimeout: 30 * time.Second,
	}
}
