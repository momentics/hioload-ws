// File: client/client.go
// Package client provides a zero-copy, batch-capable, reconnecting WebSocket client.
// Author: momentics <momentics.com>
// License: Apache-2.0
//
// This client fully implements:
// - RFC6455 WebSocket handshake over bare TCP (with or without ws:// scheme)
// - Zero-copy batch receive using NUMA-aware buffer pools
// - Masking/unmasking and framing per spec
// - Configurable read/write deadlines and optional heartbeat (Ping/Pong)
// - Automatic reconnect with backoff (controlled by ReconnectMax)
// - Lifecycle callbacks: OnConnect, OnClose, OnError
// - Idempotent Close and immediate OnConnect for handlers registered after connection

package client

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// ConnEventHandler defines lifecycle callback signatures.
type ConnEventHandler interface {
	OnConnect()
	OnClose()
	OnError(err error)
}

// ClientConfig holds all configurable parameters for the WebSocket client.
type ClientConfig struct {
	Addr              string        // WebSocket URL or bare host:port
	IOBufferSize      int           // size for zero-copy reads
	BatchSize         int           // max messages per RecvBatch
	NUMANode          int           // preferred NUMA node (-1 = auto)
	ReadTimeout       time.Duration // read deadline
	WriteTimeout      time.Duration // write deadline
	ReconnectMax      int           // max reconnect attempts (0 = no retries)
	HeartbeatInterval time.Duration // send ping every interval (0 = disabled)
}

// WebSocketClient implements a zero-copy, batch-capable, reconnecting client.
type WebSocketClient struct {
	cfg       ClientConfig
	bufPool   api.BufferPool
	transport api.Transport

	recvChan  chan *protocol.WSFrame
	closeChan chan struct{}

	mu        sync.Mutex
	handlers  []ConnEventHandler
	connected atomic.Bool
	closed    atomic.Bool
	attempts  int
}

// NewWebSocketClient constructs and connects a new WebSocketClient.
// It blocks until the initial handshake completes or fails.
func NewWebSocketClient(cfg ClientConfig) (*WebSocketClient, error) {
	mgr := pool.NewBufferPoolManager()
	bufPool := mgr.GetPool(cfg.NUMANode)

	c := &WebSocketClient{
		cfg:       cfg,
		bufPool:   bufPool,
		recvChan:  make(chan *protocol.WSFrame, cfg.BatchSize),
		closeChan: make(chan struct{}),
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// RegisterHandler adds a lifecycle event handler.
// If already connected, invokes OnConnect immediately.
func (c *WebSocketClient) RegisterHandler(h ConnEventHandler) {
	c.mu.Lock()
	c.handlers = append(c.handlers, h)
	already := c.connected.Load()
	c.mu.Unlock()
	if already {
		go h.OnConnect()
	}
}

// SendFrame masks and sends a single WSFrame.
func (c *WebSocketClient) SendFrame(frame *protocol.WSFrame) error {
	// Allocate header + payload buffer
	buf := make([]byte, protocol.MaxFrameHeaderLen+int(frame.PayloadLen))
	n, err := protocol.EncodeFrame(buf, frame.Opcode, frame.Payload, true)
	if err != nil {
		return err
	}
	// Apply write deadline if supported
	if w, ok := c.transport.(interface{ SetWriteDeadline(time.Time) error }); ok {
		_ = w.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	}
	return c.transport.Send([][]byte{buf[:n]})
}

// RecvBatch returns up to BatchSize frames or waits until timeout.
// Ignores internal read timeouts to avoid premature closure.
func (c *WebSocketClient) RecvBatch() ([]*protocol.WSFrame, error) {
	var batch []*protocol.WSFrame
	timer := time.NewTimer(c.cfg.ReadTimeout)
	defer timer.Stop()
	for len(batch) < c.cfg.BatchSize {
		select {
		case f := <-c.recvChan:
			batch = append(batch, f)
		case <-timer.C:
			return batch, nil
		case <-c.closeChan:
			return batch, api.ErrTransportClosed
		}
	}
	return batch, nil
}

// Close cleanly shuts down the client; idempotent.
func (c *WebSocketClient) Close() error {
	// Only first caller proceeds
	if !c.connected.CompareAndSwap(true, false) {
		return nil
	}
	c.closed.Store(true)
	close(c.closeChan)
	_ = c.transport.Close()

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, h := range c.handlers {
		h.OnClose()
	}
	return nil
}

// connect manages dialing, handshake, and retries per ReconnectMax.
func (c *WebSocketClient) connect() error {
	var lastErr error
	for {
		// No retries configured → single attempt only
		if c.cfg.ReconnectMax == 0 && c.attempts > 0 {
			return lastErr
		}
		// Exceeded retry count
		if c.cfg.ReconnectMax > 0 && c.attempts >= c.cfg.ReconnectMax {
			return fmt.Errorf("max reconnect attempts reached: %w", lastErr)
		}
		c.attempts++
		if err := c.dialAndHandshake(); err != nil {
			lastErr = err
			if c.cfg.ReconnectMax > 0 {
				time.Sleep(time.Duration(c.attempts) * 100 * time.Millisecond)
				continue
			}
			return lastErr
		}
		return nil
	}
}

// dialAndHandshake performs one TCP dial and WebSocket HTTP Upgrade handshake.
func (c *WebSocketClient) dialAndHandshake() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Parse address: accept ws:// or bare host:port
	addr := c.cfg.Addr
	var host, path string
	if strings.Contains(addr, "://") {
		u, err := url.Parse(addr)
		if err != nil {
			return err
		}
		host = u.Host
		path = u.RequestURI()
	} else {
		host = addr
		path = "/"
	}

	conn, err := net.Dial("tcp", host)
	if err != nil {
		return err
	}

	// Build HTTP Upgrade request per RFC6455
	keyBytes := make([]byte, 16)
	rand.Read(keyBytes)
	secKey := base64.StdEncoding.EncodeToString(keyBytes)
	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, host, secKey,
	)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return err
	}

	// Read and verify Upgrade response
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake read error: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return fmt.Errorf("handshake failed: status %d", resp.StatusCode)
	}

	// Wrap transport for zero-copy I/O
	c.transport = NewClientTransport(conn, c.bufPool, c.cfg.IOBufferSize)
	c.connected.Store(true)
	c.attempts = 0

	// Notify handlers
	for _, h := range c.handlers {
		go h.OnConnect()
	}

	// Start receive loop and optional heartbeat
	go c.recvLoop()
	if c.cfg.HeartbeatInterval > 0 {
		go c.heartbeatLoop()
	}
	return nil
}

// recvLoop reads raw buffers, decodes frames, and dispatches them.
// Ignores benign timeouts and EOF; only real errors exit.
func (c *WebSocketClient) recvLoop() {
	for {
		if c.closed.Load() {
			return
		}
		// Apply read deadline if supported
		if rd, ok := c.transport.(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = rd.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
		}
		raws, err := c.transport.Recv()
		if err != nil {
			// Ignore timeouts
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			// Ignore EOF
			if err == io.EOF {
				continue
			}
			// On real errors, notify and exit loop
			for _, h := range c.handlers {
				h.OnError(err)
			}
			return
		}
		// Decode and dispatch frames
		for _, raw := range raws {
			frame, ferr := protocol.DecodeFrameFromBytes(raw)
			if ferr != nil {
				continue
			}
			select {
			case c.recvChan <- frame:
			case <-c.closeChan:
				return
			}
		}
	}
}

// heartbeatLoop sends Ping frames at the configured interval.
func (c *WebSocketClient) heartbeatLoop() {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = c.SendFrame(&protocol.WSFrame{IsFinal: true, Opcode: protocol.OpcodePing})
		case <-c.closeChan:
			return
		}
	}
}
