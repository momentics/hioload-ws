package server

import (
	"time"

	"github.com/momentics/hioload-ws/api"
)

// Config holds all server-side configuration parameters.
type Config struct {
	ListenAddr      string        // TCP bind address, e.g. ":9000"
	IOBufferSize    int           // size of zero-copy I/O buffers
	ChannelCapacity int           // capacity of each WSConnection channel
	NUMANode        int           // preferred NUMA node (-1 = auto)
	ReadTimeout     time.Duration // optional per-connection read deadline
	WriteTimeout    time.Duration // optional per-connection write deadline
	ShutdownTimeout time.Duration // graceful shutdown timeout
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:      ":9000",
		IOBufferSize:    64 * 1024,
		ChannelCapacity: 64,
		NUMANode:        -1,
		ReadTimeout:     0,
		WriteTimeout:    0,
		ShutdownTimeout: 30 * time.Second,
	}
}

// Server is the high-level façade encapsulating listener, pool, and control.
type Server struct {
	cfg      *Config
	pool     api.BufferPool
	control  api.Control
	listener *Listener
	shutdown chan struct{}
}
