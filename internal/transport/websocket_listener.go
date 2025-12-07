// File: internal/transport/websocket_listener.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// NUMA-aware, zero-copy, cross-platform WebSocketListener compatible with BufferPoolManager and
// concurrency threading contracts. Uses explicit NUMA node via option, auto-detection fallback.
//
// Handles accept, handshake, and wraps connection in pooled zero-copy transport.

package transport

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// ListenerOption allows config (NUMA node selection, pool override, etc).
type ListenerOption func(*WebSocketListener)

// WithListenerNUMANode applies a specific NUMA node and associated buffer pool.
// Reads NUMA nodes count from the concurrency module before constructing BufferPoolManager.
func WithListenerNUMANode(node int) ListenerOption {
	return func(wsl *WebSocketListener) {
		numaCount := concurrency.NUMANodes()
		wsl.numaNode = node
		wsl.bufferPool = pool.NewBufferPoolManager(numaCount).GetPool(4096, node)
	}
}

// WebSocketListener is a TCP->WebSocket handshake acceptor, NUMA-aware.
type WebSocketListener struct {
	listener    net.Listener
	bufferPool  api.BufferPool
	channelSize int
	numaNode    int
	closed      bool
}

// NewWebSocketListener binds TCP and configures NUMA-aware pools.
func NewWebSocketListener(addr string, bufPool api.BufferPool, channelSize int, opts ...ListenerOption) (*WebSocketListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	wsl := &WebSocketListener{
		listener:    ln,
		bufferPool:  bufPool,
		channelSize: channelSize,
		numaNode:    0,
	}
	for _, opt := range opts {
		opt(wsl)
	}
	return wsl, nil
}

// Accept TCP and perform strict WebSocket RFC6455 handshake.
func (wsl *WebSocketListener) Accept() (*protocol.WSConnection, error) {
	if wsl.closed {
		return nil, ErrListenerClosed
	}
	// fmt.Println("DEBUG: Server Accept waiting for connection")
	tcpConn, err := wsl.listener.Accept()
	if err != nil {
		if strings.Contains(err.Error(), "closed network connection") {
			return nil, ErrListenerClosed
		}
		return nil, err
	}
	// fmt.Println("DEBUG: Server Accept got connection")
	
	// Disable Nagle's algorithm for low-latency small packet transmission
	if tc, ok := tcpConn.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}

	// Use buffered handshake to preserve any data read after HTTP headers
	hdrs, path, br, err := protocol.DoHandshakeCoreBuffered(tcpConn)
	if err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake request failed: %w", err)
	}
	// fmt.Println("DEBUG: Server handshake request parsed")
	if err := protocol.WriteHandshakeResponse(tcpConn, hdrs); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake response failed: %w", err)
	}
	// fmt.Println("DEBUG: Server handshake response written")

	// Use buffered transport to preserve any data buffered during handshake
	tr := &bufferedConnTransport{
		conn:       tcpConn,
		br:         br,
		bufferPool: wsl.bufferPool,
		numaNode:   wsl.numaNode,
	}
	wsConn := protocol.NewWSConnectionWithPath(tr, wsl.bufferPool, wsl.channelSize, path)
	return wsConn, nil
}

// Close listener.
func (wsl *WebSocketListener) Close() error {
	if wsl.closed {
		return nil
	}
	wsl.closed = true
	return wsl.listener.Close()
}

var ErrListenerClosed = errors.New("listener closed")

// bufferedConnTransport implements api.Transport over net.Conn with a bufio.Reader
// to preserve any data buffered during handshake.
type bufferedConnTransport struct {
	conn       net.Conn
	br         *bufio.Reader
	bufferPool api.BufferPool
	numaNode   int
	closed     bool
}

func (t *bufferedConnTransport) Send(buffers [][]byte) error {
	if t.closed {
		return api.ErrTransportClosed
	}
	for _, b := range buffers {
		if _, err := t.conn.Write(b); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	return nil
}

func (t *bufferedConnTransport) Recv() ([][]byte, error) {
	if t.closed {
		return nil, api.ErrTransportClosed
	}
	// Allocate buffer from concrete NUMA node pool.
	buf := t.bufferPool.Get(8192, t.numaNode) // Increased from 4096 for efficiency
	data := buf.Bytes()
	// Read from buffered reader to get any data buffered during handshake
	n, err := t.br.Read(data)
	if err != nil {
		buf.Release()
		return nil, fmt.Errorf("read: %w", err)
	}
	return [][]byte{data[:n]}, nil
}

func (t *bufferedConnTransport) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	return t.conn.Close()
}

func (t *bufferedConnTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     false,
		NUMAAware: true,
	}
}
