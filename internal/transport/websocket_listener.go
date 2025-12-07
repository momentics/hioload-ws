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
	tcpConn, err := wsl.listener.Accept()
	if err != nil {
		if strings.Contains(err.Error(), "closed network connection") {
			return nil, ErrListenerClosed
		}
		return nil, err
	}

	// Extract request path during handshake
	hdrs, path, err := protocol.DoHandshakeCoreWithPath(tcpConn)
	if err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake request failed: %w", err)
	}
	if err := protocol.WriteHandshakeResponse(tcpConn, hdrs); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake response failed: %w", err)
	}
	if err := protocol.WriteHandshakeResponse(tcpConn, hdrs); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake response failed: %w", err)
	}

	// Upgrade to optimized transport if possible
	tf := NewTransportFactory(4096, wsl.numaNode)
	tr, err := tf.CreateFromConn(tcpConn)
	if err != nil {
		// Fallback
		fmt.Printf("DEBUG: Server transport upgrade failed: %v. Using default.\n", err)
		tr = &connTransport{
			conn:       tcpConn,
			bufferPool: wsl.bufferPool,
			numaNode:   wsl.numaNode,
		}
	} else {
		// fmt.Printf("DEBUG: Server successfully upgraded to optimized transport.\n")
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

// connTransport implements api.Transport over net.Conn, NUMA-aware and pooled.
type connTransport struct {
	conn       net.Conn
	bufferPool api.BufferPool
	numaNode   int
	closed     bool
}

func (t *connTransport) Send(buffers [][]byte) error {
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
func (t *connTransport) Recv() ([][]byte, error) {
	if t.closed {
		return nil, api.ErrTransportClosed
	}
	// Allocate buffer from concrete NUMA node pool.
	buf := t.bufferPool.Get(4096, t.numaNode)
	data := buf.Bytes()
	n, err := t.conn.Read(data)
	if err != nil {
		buf.Release()
		return nil, fmt.Errorf("read: %w", err)
	}
	return [][]byte{data[:n]}, nil
}
func (t *connTransport) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	return t.conn.Close()
}
func (t *connTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     false,
		NUMAAware: true,
	}
}
