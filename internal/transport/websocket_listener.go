// File: internal/transport/websocket_listener.go
// Direct WebSocket listener without HTTP overhead.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0//
// This implementation has been updated to eliminate extra payload copying.
// Now it returns zero-copy pooled Buffers directly from Recv(), instead of
// copying into a temporary byte slice.

package transport

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// ErrListenerClosed is returned by Accept when the listener has been closed.
// This sentinel error is used to signal graceful shutdown.
var ErrListenerClosed = errors.New("listener closed")

// WebSocketListener provides direct native WebSocket connections.
type WebSocketListener struct {
	listener    net.Listener
	bufferPool  api.BufferPool
	channelSize int
	closed      bool
}

// NewWebSocketListener creates a new WebSocketListener with strict header limits.
// It initializes a TCP listener and retains the given buffer pool and channel size
// for all accepted WebSocket connections.
func NewWebSocketListener(addr string, bufPool api.BufferPool, channelSize int) (*WebSocketListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return &WebSocketListener{
		listener:    ln,
		bufferPool:  bufPool,
		channelSize: channelSize,
	}, nil
}

// Accept returns a ready WSConnection after completing handshake.
// Eliminates any payload copy by reading frames natively into pooled Buffers.
func (wsl *WebSocketListener) Accept() (*protocol.WSConnection, error) {
	if wsl.closed {
		return nil, ErrListenerClosed
	}
	conn, err := wsl.listener.Accept()
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
			return nil, ErrListenerClosed
		}
		return nil, fmt.Errorf("accept connection: %w", err)
	}
	// Perform handshake core (parses client headers, computes Sec-WebSocket-Accept)
	hdrs, err := protocol.DoHandshakeCore(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake core: %w", err)
	}
	// Write response headers
	var sb strings.Builder
	sb.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	for k, vs := range hdrs {
		for _, v := range vs {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}
	sb.WriteString("\r\n")
	if _, err := conn.Write([]byte(sb.String())); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write handshake response: %w", err)
	}
	// Wrap the net.Conn in our zero-copy connTransport
	tr := &connTransport{conn: conn, bufferPool: wsl.bufferPool}
	return protocol.NewWSConnection(tr, wsl.bufferPool, wsl.channelSize), nil
}

// Close shuts down the listener to stop Accept.
// After Close, Accept returns ErrListenerClosed.
func (wsl *WebSocketListener) Close() error {
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
// The Recv method no longer allocates intermediate []byte slices;
// it reads directly into a pooled Buffer and returns it for zero-copy processing.
type connTransport struct {
	conn       net.Conn
	bufferPool api.BufferPool
	closed     bool
}

// Send writes each buffer in full directly to the connection.
// No change here: buffers come from pool or framing logic.
func (ct *connTransport) Send(buffers [][]byte) error {
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

// Recv performs zero-copy read: allocates a pooled Buffer, reads from conn
// directly into the buffer's Bytes(), and returns the Buffer without copying.
func (ct *connTransport) Recv() ([][]byte, error) {
	if ct.closed {
		return nil, api.ErrTransportClosed
	}
	// Allocate a single buffer of default read size (e.g., 4096)
	b := ct.bufferPool.Get(4096, -1)  // get pooled Buffer
	n, err := ct.conn.Read(b.Bytes()) // read directly into pooled memory
	if err != nil {
		b.Release()
		return nil, fmt.Errorf("read from connection: %w", err)
	}
	// Return the slice of the buffer with only the valid bytes
	// Note: we return b.Bytes()[:n]; the ownership of b remains with caller via bufferPool.
	return [][]byte{b.Bytes()[:n]}, nil
}

// Close marks the transport as closed and closes the underlying connection.
func (ct *connTransport) Close() error {
	if ct.closed {
		return nil
	}
	ct.closed = true
	return ct.conn.Close()
}

// Features reports the capabilities of this transport:
// zero-copy at the application boundary (we read directly into pooled buffers).
func (ct *connTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     false,
		NUMAAware: false,
	}
}
