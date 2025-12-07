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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
	pool "github.com/momentics/hioload-ws/core/buffer"
	"github.com/momentics/hioload-ws/core/concurrency"
	internal_transport "github.com/momentics/hioload-ws/internal/transport"
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

	// Setup buffer pool manager
	mgr := pool.NewBufferPoolManager(concurrency.NUMANodes())

	// Try optimized "raw" client first (avoids Windows IOCP conflict)
	tf := internal_transport.NewTransportFactory(cfg.IOBufferSize, cfg.NUMANode)
	tr, err := tf.CreateClient(u.Host)

	if err == nil {
		fmt.Printf("DEBUG: Created optimized raw client transport.\n")

		// We have a transport, but we need to do the HTTP handshake.
		// Construct a dummy net.Conn adapter for the protocol handshake functions.
		adapter := &transportAdapter{
			tr: tr,
		}

		// Handshake
		key := make([]byte, 16)
		rand.Read(key)
		secKey := base64.StdEncoding.EncodeToString(key)
		reqStr := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", u.Path, u.Host, secKey)

		// Send request directly
		if err := adapter.WriteRaw([]byte(reqStr)); err != nil {
			tr.Close()
			return nil, fmt.Errorf("handshake write failed: %w", err)
		}

		// Read response using the helper which expects bufio-like behavior or just Read()
		// protocol.DoClientHandshake expects a net.Conn to Read/Write.
		// Let's use our adapter.

		// Actually, DoClientHandshake does validation.
		// We can just manually validate the response here to be simpler and strictly use our transport.
		// Read response
		respBuf := make([]byte, 4096)
		n, err := adapter.Read(respBuf)
		if err != nil {
			tr.Close()
			return nil, fmt.Errorf("handshake read failed: %w", err)
		}
		respStr := string(respBuf[:n])
		if !strings.Contains(respStr, "101 Switching Protocols") {
			tr.Close()
			return nil, fmt.Errorf("handshake failed, invalid response: %s", respStr)
		}

	} else {
		// Fallback to net.Dial
		fmt.Printf("DEBUG: Optimized client creation failed: %v. Using net.Dial.\n", err)
		netConn, err := net.Dial("tcp", u.Host)
		if err != nil {
			return nil, fmt.Errorf("dial error: %w", err)
		}

		// Perform HTTP handshake on net.Conn
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

		// Wrap
		tr = NewTransport(netConn, mgr.GetPool(cfg.IOBufferSize, cfg.NUMANode), cfg.IOBufferSize)
	}

	// Setup buffer pool (reuse existing manager)
	bp := mgr.GetPool(cfg.IOBufferSize, cfg.NUMANode)

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

// transportAdapter adapts api.Transport to io.ReadWriter for handshake
type transportAdapter struct {
	tr     api.Transport
	excess []byte
}

func (t *transportAdapter) Read(p []byte) (n int, err error) {
	if len(t.excess) > 0 {
		n = copy(p, t.excess)
		t.excess = t.excess[n:]
		return n, nil
	}
	bufs, err := t.tr.Recv()
	if err != nil {
		return 0, err
	}
	if len(bufs) == 0 {
		return 0, nil
	}
	// Copy first buffer
	n = copy(p, bufs[0])
	if n < len(bufs[0]) {
		t.excess = bufs[0][n:]
	}
	// Warning: dropping other buffers if batch > 1. Handshake shouldn't be batched ideally.
	return n, nil
}

func (t *transportAdapter) Write(p []byte) (n int, err error) {
	// Copy p because Transport expects to own the buffer potentially or we just send copy
	// Send expects [][]byte.
	// We need to match behavior: Transport.Send writes buffers.
	cp := make([]byte, len(p))
	copy(cp, p)
	err = t.tr.Send([][]byte{cp})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *transportAdapter) WriteRaw(p []byte) error {
	_, err := t.Write(p)
	return err
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

// ReadMessage reads a single message from the connection.
func (c *Client) ReadMessage() (messageType int, p []byte, err error) {
	buffers, err := c.conn.RecvZeroCopy()
	if err != nil {
		return 0, nil, err
	}

	if len(buffers) == 0 {
		return 0, nil, fmt.Errorf("no message received")
	}

	// Get the first buffer
	buf := buffers[0]
	data := buf.Bytes()

	// Create a copy of the data to return
	result := make([]byte, len(data))
	copy(result, data)

	// Release the buffer (important for zero-copy semantics)
	buf.Release()

	// For now, return binary type - in real implementation we'd get this from frame
	return int(protocol.OpcodeBinary), result, nil
}

// WriteMessage writes a message to the connection.
func (c *Client) WriteMessage(messageType int, data []byte) error {
	// Get a buffer from the connection's buffer pool for zero-copy sending
	buf := c.conn.BufferPool().Get(len(data), -1) // Use the connection's buffer pool

	// Copy data to the buffer
	dest := buf.Bytes()
	copy(dest, data)

	// Create a frame - for client, frames must be masked per RFC 6455
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     byte(messageType),
		Masked:     true, // Client frames must be masked per RFC 6455
		PayloadLen: int64(len(data)),
		Payload:    dest[:len(data)],
	}

	// Send the frame
	err := c.conn.SendFrame(frame)

	// Release the buffer after sending
	buf.Release()

	return err
}

// ReadJSON unmarshals the next JSON message from the connection.
func (c *Client) ReadJSON(v interface{}) error {
	_, payload, err := c.ReadMessage()
	if err != nil {
		return err
	}

	// Unmarshal JSON directly from the payload
	return json.Unmarshal(payload, v)
}

// WriteJSON marshals and sends a JSON message over the connection.
func (c *Client) WriteJSON(v interface{}) error {
	// Marshal to JSON first
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// Use WriteMessage to send as binary
	return c.WriteMessage(int(protocol.OpcodeBinary), data)
}

// Close gracefully shuts down I/O and underlying connection.
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	c.conn.Close()
	return nil
}

// GetWSConnection returns the underlying WebSocket connection.
func (c *Client) GetWSConnection() *protocol.WSConnection {
	return c.conn
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
