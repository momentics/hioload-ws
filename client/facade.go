// Package client provides a unified, zero-copy, NUMA-aware WebSocket client façade.
// It encapsulates connection setup, handshake, batch send/recv loops, and graceful shutdown.
//
// Core design:
// - Zero-copy payloads via BufferPool
// - Batch I/O (batchSize) for maximum throughput
// - NUMA-aware buffer allocation per configured node
// - Lock-free batch enqueue/dequeue for outgoing frames
// - Cross-platform transport abstraction (epoll/IOCP/RIO)
package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// Config holds client parameters for high-performance connections.
type Config struct {
	Addr         string        // WebSocket URL (ws://host:port/path)
	IOBufferSize int           // size per zero-copy buffer
	BatchSize    int           // number of frames per batch
	NUMANode     int           // preferred NUMA node (-1 = auto)
	ReadTimeout  time.Duration // per-recv deadline, 0 = disabled
	WriteTimeout time.Duration // per-send deadline, 0 = disabled
	Heartbeat    time.Duration // Ping interval, 0 = disabled
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Addr:         "ws://localhost:9000",
		IOBufferSize: 64 * 1024,
		BatchSize:    16,
		NUMANode:     -1,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Heartbeat:    30 * time.Second,
	}
}

// Client is a high-level WebSocket client.
type Client struct {
	cfg       *Config
	transport api.Transport
	conn      *protocol.WSConnection
	sendBatch *Batch
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewClient initializes, handshakes, and starts I/O loops.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Parse URL
	u, err := url.Parse(cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Dial TCP
	netConn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	// Perform HTTP handshake
	key := make([]byte, 16)
	rand.Read(key)
	secKey := base64.StdEncoding.EncodeToString(key)
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: u.Path},
		Host:   u.Host,
		Header: http.Header{
			"Upgrade":               {"websocket"},
			"Connection":            {"Upgrade"},
			"Sec-WebSocket-Key":     {secKey},
			"Sec-WebSocket-Version": {"13"},
		},
	}
	if err := protocol.WriteHandshakeRequest(netConn, req); err != nil {
		netConn.Close()
		return nil, err
	}
	if err := protocol.DoClientHandshake(netConn, req); err != nil {
		netConn.Close()
		return nil, err
	}

	// Setup buffer pool and transport
	mgr := pool.NewBufferPoolManager(concurrency.NUMANodes())
	bp := mgr.GetPool(cfg.IOBufferSize, cfg.NUMANode)
	tr := NewTransport(netConn, bp, cfg.IOBufferSize)

	// Build WSConnection
	ws := protocol.NewWSConnection(tr, bp, cfg.BatchSize)
	ws.Start()

	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		cfg:       cfg,
		transport: tr,
		conn:      ws,
		sendBatch: NewBatch(cfg.BatchSize),
		ctx:       ctx,
		cancel:    cancel,
	}
	client.wg.Add(2)
	go client.sendLoop()
	go client.recvLoop()
	if cfg.Heartbeat > 0 {
		client.wg.Add(1)
		go client.heartbeatLoop()
	}
	return client, nil
}

// Send enqueues a binary message for batch transmission.
func (c *Client) Send(msg []byte) {
	buf := c.conn.Transport().(interface{ GetBuffer() api.Buffer }).GetBuffer()
	copy(buf.Bytes(), msg)
	c.sendBatch.Append(buf)
	if c.sendBatch.Len() >= c.cfg.BatchSize {
		c.flush()
	}
}

// Recv returns next batch of frames or error.
func (c *Client) Recv() ([]api.Buffer, error) {
	// Blocking or context-aware recv
	select {
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	default:
	}
	return c.conn.RecvZeroCopy()
}

// Close gracefully shuts down I/O and underlying connection.
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	c.conn.Close()
	return nil
}

// sendLoop flushes batches on context cancellation or flush triggers.
func (c *Client) sendLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			c.flush()
			return
		case <-ticker.C:
			c.flush()
		}
	}
}

// flush sends the current batch.
func (c *Client) flush() {
	batch := c.sendBatch.Swap()
	if len(batch) == 0 {
		return
	}
	var bufs [][]byte
	for _, b := range batch {
		bufs = append(bufs, b.Bytes())
	}
	if err := c.transport.Send(bufs); err != nil {
		// handle error/log
	}
	for _, b := range batch {
		b.Release()
	}
}

// recvLoop handles read timeouts and incoming control frames.
func (c *Client) recvLoop() {
	defer c.wg.Done()
	for {
		if c.cfg.ReadTimeout > 0 {
			if rd, ok := c.transport.(interface{ SetReadDeadline(time.Time) error }); ok {
				rd.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
			}
		}
		_, err := c.conn.RecvZeroCopy()
		if err != nil {
			return
		}
		// deliver to application...
	}
}

// heartbeatLoop sends periodic ping frames.
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.Heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.conn.SendFrame(&protocol.WSFrame{IsFinal: true, Opcode: protocol.OpcodePing})
		}
	}
}
