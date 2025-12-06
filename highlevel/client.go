// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
package highlevel

import (
	"net/url"
	"time"

	"github.com/momentics/hioload-ws/lowlevel/client"
	"github.com/momentics/hioload-ws/core/buffer"
	"github.com/momentics/hioload-ws/core/concurrency"
)

// Dial connects to a WebSocket server.
func Dial(rawurl string) (*Conn, error) {
	return DialWithOptions(rawurl)
}

// DialOption configures a Dial operation.
type DialOption func(*dialConfig)

type dialConfig struct {
	timeout          time.Duration
	handshakeTimeout time.Duration
	readLimit        int64
	numaNode         int
	batchSize        int
	ioBufferSize     int
}

// DialWithOptions connects to a WebSocket server with specific options.
func DialWithOptions(rawurl string, opts ...DialOption) (*Conn, error) {
	_, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}

	cfg := &dialConfig{
		timeout:          45 * time.Second, // default timeout
		handshakeTimeout: 45 * time.Second, // default handshake timeout
		readLimit:        32 << 20,        // 32MB default read limit
		numaNode:         -1,              // auto-detect
		batchSize:        16,              // default batch size
		ioBufferSize:     64 * 1024,        // 64KB default buffer size
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Use hioload-ws client facade to establish connection
	clientCfg := &client.Config{
		Addr:         rawurl,
		IOBufferSize: cfg.ioBufferSize,
		BatchSize:    cfg.batchSize,
		NUMANode:     cfg.numaNode,
		ReadTimeout:  cfg.timeout,
		WriteTimeout: cfg.timeout,
		Heartbeat:    30 * time.Second, // default heartbeat
	}

	// Create the client using hioload-ws primitives
	hioloadClient, err := client.NewClient(clientCfg)
	if err != nil {
		return nil, err
	}

	// Create buffer pool that matches the client configuration
	nodeCount := concurrency.NUMANodes()
	bufMgr := pool.NewBufferPoolManager(nodeCount)
	bufPool := bufMgr.GetPool(cfg.ioBufferSize, cfg.numaNode)

	// Create connection with proper linking to the underlying client
	conn := &Conn{
		pool:         bufPool,
		readLimit:    cfg.readLimit,
		readTimeout:  cfg.timeout,
		writeTimeout: cfg.timeout,
		autoRelease:  true,
		client:       hioloadClient,
	}

	return conn, nil
}

// WithDialTimeout sets the timeout for the dial operation.
func WithClientDialTimeout(d time.Duration) DialOption {
	return func(c *dialConfig) {
		c.timeout = d
	}
}

// WithClientHandshakeTimeout sets the timeout for the handshake operation.
func WithClientHandshakeTimeout(d time.Duration) DialOption {
	return func(c *dialConfig) {
		c.handshakeTimeout = d
	}
}

// WithClientReadLimit sets the maximum size for incoming messages.
func WithClientReadLimit(limit int64) DialOption {
	return func(c *dialConfig) {
		c.readLimit = limit
	}
}

// WithClientNUMANode sets the preferred NUMA node for this connection.
func WithClientNUMANode(node int) DialOption {
	return func(c *dialConfig) {
		c.numaNode = node
	}
}

// WithClientBatchSize sets the batch size for this connection.
func WithClientBatchSize(size int) DialOption {
	return func(c *dialConfig) {
		c.batchSize = size
	}
}