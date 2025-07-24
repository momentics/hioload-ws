// File: client/client.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Provides a zero-copy, batch IO, reconnecting WebSocket client
// for ultra-high-load testing with NUMA-aware buffer pools.
//
// This version explicitly defines ClientConfig and ConnEventHandler
// outside the main struct, making them visible for users/importers.

package client

import (
	"bufio"
	"context"
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

// ClientConfig defines the configuration parameters for creating a WebSocket client.
//
// This struct is fully documented and intentionally placed at the package level
// so that it can be directly imported from client for integration in test programs.
type ClientConfig struct {
	Addr              string        // Full WebSocket URL or "host:port"
	IOBufferSize      int           // Zero-copy receive buffer size
	BatchSize         int           // Number of frames processed in one batch
	NUMANode          int           // Preferred NUMA node (-1: automatic)
	ReadTimeout       time.Duration // Per-read operation deadline
	WriteTimeout      time.Duration // Per-write operation deadline
	ReconnectMax      int           // Max reconnect attempts (0=disabled)
	HeartbeatInterval time.Duration // Heartbeat (Ping) interval (0 = off)
}

// ConnEventHandler exposes optional protocol callbacks for connection lifecycle events.
//
// Implementers can log events or react to errors in a scalable test harness.
// If not required, pass nil or embed empty implementations.
type ConnEventHandler interface {
	OnConnect()
	OnClose()
	OnError(error)
}

// ClientOption allows for functional-style configuration.
type ClientOption func(*WebSocketClient)

// WithDialer allows for customized outgoing connections (for IP aliasing).
func WithDialer(d *net.Dialer) ClientOption {
	return func(c *WebSocketClient) {
		if d != nil {
			c.dialer = d
		}
	}
}

// WebSocketClient implements the core, NUMA-aware WebSocket stress client.
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

	dialer *net.Dialer // Optionally, use non-default dialer
}

// NewWebSocketClient constructs a client with full zero-copy, batch, affinity support.
// Accepts functional options for dialing customization.
func NewWebSocketClient(cfg ClientConfig, opts ...ClientOption) (*WebSocketClient, error) {
	mgr := pool.NewBufferPoolManager()
	bufPool := mgr.GetPool(cfg.NUMANode)

	c := &WebSocketClient{
		cfg:       cfg,
		bufPool:   bufPool,
		recvChan:  make(chan *protocol.WSFrame, cfg.BatchSize),
		closeChan: make(chan struct{}),
	}
	// Apply all user options before connect
	for _, opt := range opts {
		opt(c)
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// RegisterHandler allows for connection lifecycle event reporting.
func (c *WebSocketClient) RegisterHandler(h ConnEventHandler) {
	c.mu.Lock()
	c.handlers = append(c.handlers, h)
	already := c.connected.Load()
	c.mu.Unlock()
	if already {
		go h.OnConnect()
	}
}

// SendFrame transmits a single WebSocket frame (encoded with protocol masking and RFC6455 header).
func (c *WebSocketClient) SendFrame(frame *protocol.WSFrame) error {
	buf := make([]byte, protocol.MaxFrameHeaderLen+int(frame.PayloadLen))
	n, err := protocol.EncodeFrame(buf, frame.Opcode, frame.Payload, true)
	if err != nil {
		return err
	}
	if w, ok := c.transport.(interface{ SetWriteDeadline(time.Time) error }); ok {
		_ = w.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	}
	// Batch API, but here always length 1
	return c.transport.Send([][]byte{buf[:n]})
}

// RecvBatch returns up to BatchSize frames (or less, if timeout triggers).
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

// Close idempotently terminates the connection, notifying all handlers.
func (c *WebSocketClient) Close() error {
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

// --- Connection Management Internals ---

func (c *WebSocketClient) connect() error {
	var lastErr error
	for {
		if c.cfg.ReconnectMax == 0 && c.attempts > 0 {
			return lastErr
		}
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

// dialAndHandshake supports both default and custom dialers for advanced connection management.
func (c *WebSocketClient) dialAndHandshake() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	var conn net.Conn
	var err error
	if c.dialer != nil {
		ctx := context.Background()
		conn, err = c.dialer.DialContext(ctx, "tcp", host)
	} else {
		conn, err = net.Dial("tcp", host)
	}
	if err != nil {
		return err
	}

	// Compose RFC6455 upgrade request
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

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake read error: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return fmt.Errorf("handshake failed: status %d", resp.StatusCode)
	}

	c.transport = NewClientTransport(conn, c.bufPool, c.cfg.IOBufferSize)
	c.connected.Store(true)
	c.attempts = 0

	for _, h := range c.handlers {
		go h.OnConnect()
	}

	go c.recvLoop()
	if c.cfg.HeartbeatInterval > 0 {
		go c.heartbeatLoop()
	}
	return nil
}

// recvLoop continuously receives and decodes frames, pushing them to recvChan.
func (c *WebSocketClient) recvLoop() {
	for {
		if c.closed.Load() {
			return
		}
		if rd, ok := c.transport.(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = rd.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
		}
		raws, err := c.transport.Recv()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if err == io.EOF {
				continue
			}
			for _, h := range c.handlers {
				h.OnError(err)
			}
			return
		}
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

// heartbeatLoop optionally sends periodic Ping frames if so configured.
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
