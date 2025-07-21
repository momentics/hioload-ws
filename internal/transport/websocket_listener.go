// File: internal/transport/websocket_listener.go
// Direct WebSocket listener without HTTP overhead.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package transport

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// WebSocketListener provides direct native WebSocket connections.
type WebSocketListener struct {
	listener    net.Listener
	bufferPool  api.BufferPool
	channelSize int
	mu          sync.RWMutex
	closed      bool
}

// NewWebSocketListener creates a new WebSocketListener with strict header limits.
func NewWebSocketListener(addr string, bufPool api.BufferPool, channelSize int) (*WebSocketListener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return &WebSocketListener{
		listener:    listener,
		bufferPool:  bufPool,
		channelSize: channelSize,
	}, nil
}

// Accept returns a ready WSConnection after validating and completing handshake.
func (wsl *WebSocketListener) Accept() (*protocol.WSConnection, error) {
	wsl.mu.RLock()
	if wsl.closed {
		wsl.mu.RUnlock()
		return nil, fmt.Errorf("listener closed")
	}
	wsl.mu.RUnlock()

	conn, err := wsl.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept connection: %w", err)
	}

	wsConn, err := wsl.performHandshake(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake: %w", err)
	}

	return wsConn, nil
}

// performHandshake handles HTTP upgrade parsing and calls protocol.UpgradeToWebSocket.
func (wsl *WebSocketListener) performHandshake(conn net.Conn) (*protocol.WSConnection, error) {
	reader := bufio.NewReader(conn)
	// Read request line
	reqLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read request line: %w", err)
	}
	reqLine = strings.TrimRight(reqLine, "\r\n")
	if !strings.HasPrefix(reqLine, "GET ") {
		return nil, fmt.Errorf("invalid request method: %s", reqLine)
	}

	// Parse headers
	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			headers[key] = val
		}
	}

	// Enforce max header bytes
	total := 0
	for k, v := range headers {
		total += len(k) + len(v)
		if total > protocol.MaxHandshakeHeadersSize {
			return nil, fmt.Errorf("handshake headers too large")
		}
	}

	// Validate upgrade headers via protocol.ValidateUpgradeHeaders
	if err := protocol.ValidateUpgradeHeaders(headers); err != nil {
		return nil, err
	}

	// Compute Sec-WebSocket-Accept
	key := headers[protocol.HeaderSecWebSocketKey]
	accept := protocol.ComputeAcceptKey(key)

	// Write response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n\r\n", accept)
	if _, err := conn.Write([]byte(response)); err != nil {
		return nil, fmt.Errorf("write upgrade response: %w", err)
	}

	// Wrap raw connection into api.Transport implementation
	tr := &connTransport{conn: conn, bufferPool: wsl.bufferPool}
	// Correctly call protocol.NewWSConnection with api.Transport
	return protocol.NewWSConnection(tr, wsl.bufferPool, wsl.channelSize), nil
}

// Close shuts down the listener.
func (wsl *WebSocketListener) Close() error {
	wsl.mu.Lock()
	defer wsl.mu.Unlock()
	if wsl.closed {
		return nil
	}
	wsl.closed = true
	return wsl.listener.Close()
}

// Addr returns the listener address.
func (wsl *WebSocketListener) Addr() net.Addr {
	return wsl.listener.Addr()
}

// connTransport is a zero-copy wrapper implementing api.Transport over net.Conn.
type connTransport struct {
	conn       net.Conn
	bufferPool api.BufferPool
	mu         sync.RWMutex
	closed     bool
}

func (ct *connTransport) Send(buffers [][]byte) error {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	if ct.closed {
		return api.ErrTransportClosed
	}
	for _, buf := range buffers {
		if _, err := ct.conn.Write(buf); err != nil {
			return fmt.Errorf("write to connection: %w", err)
		}
	}
	return nil
}

func (ct *connTransport) Recv() ([][]byte, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	if ct.closed {
		return nil, api.ErrTransportClosed
	}
	b := ct.bufferPool.Get(4096, -1)
	n, err := ct.conn.Read(b.Bytes())
	if err != nil {
		b.Release()
		return nil, fmt.Errorf("read from connection: %w", err)
	}
	data := make([]byte, n)
	copy(data, b.Bytes()[:n])
	b.Release()
	return [][]byte{data}, nil
}

func (ct *connTransport) Close() error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if ct.closed {
		return nil
	}
	ct.closed = true
	return ct.conn.Close()
}

func (ct *connTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: false,
		OS:        []string{"linux", "windows"},
	}
}
