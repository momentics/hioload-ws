// File: internal/transport/websocket_listener.go
// Direct WebSocket listener without HTTP overhead.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package transport

import (
	"fmt"
	"net"
	"strings"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// WebSocketListener provides direct native WebSocket connections.
type WebSocketListener struct {
	listener    net.Listener
	bufferPool  api.BufferPool
	channelSize int
	closed      bool
}

// NewWebSocketListener creates a new WebSocketListener with strict header limits.
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
func (wsl *WebSocketListener) Accept() (*protocol.WSConnection, error) {
	if wsl.closed {
		return nil, fmt.Errorf("listener closed")
	}
	conn, err := wsl.listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept connection: %w", err)
	}
	// Выполняем единый core-handshake
	hdrs, err := protocol.DoHandshakeCore(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake core: %w", err)
	}
	// Отправляем ответ
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
	// Создаём WSConnection
	tr := &connTransport{conn: conn, bufferPool: wsl.bufferPool}
	return protocol.NewWSConnection(tr, wsl.bufferPool, wsl.channelSize), nil
}

// Close shuts down the listener.
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
type connTransport struct {
	conn       net.Conn
	bufferPool api.BufferPool
	closed     bool
}

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

func (ct *connTransport) Recv() ([][]byte, error) {
	if ct.closed {
		return nil, api.ErrTransportClosed
	}
	// Single-frame read for simplicity
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
	if ct.closed {
		return nil
	}
	ct.closed = true
	return ct.conn.Close()
}

func (ct *connTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     false,
		NUMAAware: false,
	}
}
