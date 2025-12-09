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
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
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
	flushCh   chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

var encodedFramePool = sync.Pool{
	New: func() any { return make([]byte, 0, 64*1024) },
}

type slicePoolReleaser struct {
	pool *sync.Pool
}

func (s slicePoolReleaser) Put(b api.Buffer) {
	if s.pool != nil {
		s.pool.Put(b.Data[:0])
	}
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

	// Setup shared buffer pool manager
	mgr := pool.DefaultManager()

	var tr api.Transport

	// Optimized transport path is currently disabled for stability; use the Net fallback.
	netConn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	// Disable Nagle's algorithm for low-latency small packet transmission
	if tc, ok := netConn.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
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
	// Use manual string construction to match optimized path and avoid req.Write quirks
	path := u.Path
	if path == "" {
		path = "/"
	}
	reqStr := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, u.Host, secKey)

	if _, err := netConn.Write([]byte(reqStr)); err != nil {
		netConn.Close()
		return nil, err
	}

	// Set timeout for handshake response
	netConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := protocol.DoClientHandshake(netConn, req); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("fallback handshake failed: %w", err)
	}
	netConn.SetReadDeadline(time.Time{}) // Clear deadline

	// Wrap
	tr = NewTransport(netConn, mgr.GetPool(cfg.IOBufferSize, cfg.NUMANode), cfg.IOBufferSize)

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
		flushCh:   make(chan struct{}, 1),
		ctx:       ctx,
		cancel:    cancel,
	}
	client.wg.Add(1) // Only sendLoop, recvLoop is handled by WSConnection.Start()
	go client.sendLoop()
	// NOTE: Don't spawn client.recvLoop() as WSConnection.Start() already runs its own recvLoop
	// which reads from transport and pushes to inbox. Client.Recv() reads from inbox.
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
// Send enqueues a binary message for batch transmission.
func (c *Client) Send(msg []byte) {
	// Frame the message
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		Masked:     true,
		PayloadLen: int64(len(msg)),
		Payload:    msg,
	}

	scratch := encodedFramePool.Get().([]byte)
	raw, err := protocol.EncodeFrameToBufferWithMask(frame, true, scratch[:0])
	if err != nil {
		encodedFramePool.Put(scratch[:0])
		// Drop message on error (or log?)
		return
	}

	// Use custom buffer to avoid pool pollution with variable sizes
	buf := api.Buffer{Data: raw, NUMA: -1, Pool: slicePoolReleaser{pool: &encodedFramePool}}
	c.sendBatch.Append(buf)
	if c.sendBatch.Len() >= c.cfg.BatchSize {
		c.flush()
		return
	}
	c.signalFlush()
}

// frameBuffer removed as api.Buffer struct handles this case natively

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

// ReadMessage reads a single message from the connection (copying the payload).
func (c *Client) ReadMessage() (messageType int, p []byte, err error) {
	mt, buf, err := c.ReadBuffer()
	if err != nil {
		return 0, nil, err
	}
	defer buf.Release()

	data := buf.Bytes()
	result := make([]byte, len(data))
	copy(result, data)

	// For now, return binary type - in real implementation we'd get this from frame
	return mt, result, nil
}

// ReadBuffer returns the next message without copying. Caller must Release().
func (c *Client) ReadBuffer() (int, api.Buffer, error) {
	buffers, err := c.conn.RecvZeroCopy()
	if err != nil {
		return 0, api.Buffer{}, err
	}

	if len(buffers) == 0 {
		return 0, api.Buffer{}, fmt.Errorf("no message received")
	}

	return int(protocol.OpcodeBinary), buffers[0], nil
}

// WriteMessage writes a message to the connection.
func (c *Client) WriteMessage(messageType int, data []byte) error {
	// Get a buffer from the connection's buffer pool for zero-copy sending
	buf := c.conn.BufferPool().Get(len(data), -1) // Use the connection's buffer pool

	// Copy data to the buffer
	dest := buf.Bytes()
	usePool := len(dest) >= len(data)
	if usePool {
		copy(dest, data)
	} else {
		// Pool buffer too small; fall back to owned slice to avoid slicing panic
		buf.Release()
		dest = append([]byte(nil), data...)
	}

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
	if usePool {
		buf.Release()
	}

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
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			c.flush()
			return
		case <-ticker.C:
			c.flush()
		case <-c.flushCh:
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
		// fmt.Printf("DEBUG: flush Send error: %v\n", err)
		// handle error/log
	} else {
		// fmt.Printf("DEBUG: flush Sent batch size %d\n", len(batch))
	}
	for _, b := range batch {
		b.Release()
	}
}

// signalFlush requests an immediate flush without blocking the caller.
func (c *Client) signalFlush() {
	select {
	case c.flushCh <- struct{}{}:
	default:
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
