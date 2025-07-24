// File: internal/transport/websocket_listener.go
// package transport provides a native, zero-copy WebSocket listener.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// We add a synchronous handshake phase so that Accept does not return
// until the HTTP Upgrade has been fully processed. This ensures the
// client’s first SendFrame actually reaches the server-side echo loop.

package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// ErrListenerClosed is returned when Accept is called on a closed listener.
var ErrListenerClosed = errors.New("listener closed")

// WebSocketListener accepts raw TCP connections, performs the HTTP Upgrade
// handshake synchronously, then returns a fully ready *protocol.WSConnection.
type WebSocketListener struct {
	listener    net.Listener
	bufferPool  api.BufferPool
	channelSize int
	closed      bool
}

// NewWebSocketListener initializes a TCP listener for WebSocket traffic.
// channelSize dictates the size of the underlying WSConnection channels.
func NewWebSocketListener(addr string, bufPool api.BufferPool, channelSize int) (*WebSocketListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return &WebSocketListener{
		listener:    ln,
		bufferPool:  bufPool,
		channelSize: channelSize,
	}, nil
}

// Accept blocks until a raw TCP connection is accepted, then performs
// the WebSocket handshake synchronously before returning the connection.
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

	// 1. Perform synchronous HTTP Upgrade handshake core.
	hdrs, err := protocol.DoHandshakeCore(tcpConn)
	if err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake read: %w", err)
	}

	// 2. Write the response headers synchronously.
	if err := protocol.WriteHandshakeResponse(tcpConn, hdrs); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("handshake write: %w", err)
	}

	// 3. Wrap the TCP connection in a zero-copy transport.
	tr := &connTransport{conn: tcpConn, bufferPool: wsl.bufferPool}
	// 4. Construct WSConnection; now ready for SendFrame/RecvBatch.
	return protocol.NewWSConnection(tr, wsl.bufferPool, wsl.channelSize), nil
}

// Close stops the listener; subsequent Accept calls return ErrListenerClosed.
func (wsl *WebSocketListener) Close() error {
	if wsl.closed {
		return nil
	}
	wsl.closed = true
	return wsl.listener.Close()
}

// Addr returns the listener's bound address.
func (wsl *WebSocketListener) Addr() net.Addr {
	return wsl.listener.Addr()
}

// connTransport implements api.Transport by reading directly into pooled Buffers.
type connTransport struct {
	conn       net.Conn
	bufferPool api.BufferPool
	closed     bool
}

// Send writes each byte slice fully to the underlying connection.
func (ct *connTransport) Send(buffers [][]byte) error {
	if ct.closed {
		return api.ErrTransportClosed
	}
	for _, b := range buffers {
		if _, err := ct.conn.Write(b); err != nil {
			return fmt.Errorf("write: %w", err)
		}
	}
	return nil
}

// Recv allocates a single pooled Buffer, reads into it, and returns its slice.
func (ct *connTransport) Recv() ([][]byte, error) {
	if ct.closed {
		return nil, api.ErrTransportClosed
	}
	buf := ct.bufferPool.Get(4096, -1)
	data := buf.Bytes()
	n, err := ct.conn.Read(data)
	if err != nil {
		buf.Release()
		if err == io.EOF {
			return nil, api.ErrTransportClosed
		}
		return nil, fmt.Errorf("read: %w", err)
	}
	return [][]byte{data[:n]}, nil
}

// Close marks the transport closed and closes the TCP connection.
func (ct *connTransport) Close() error {
	if ct.closed {
		return nil
	}
	ct.closed = true
	return ct.conn.Close()
}

// Features advertises zero-copy and single-buffer (Batch=false) semantics.
func (ct *connTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{ZeroCopy: true, Batch: false, NUMAAware: false}
}
