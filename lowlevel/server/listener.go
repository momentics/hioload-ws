package server

import (
	"bufio"
	"fmt"
	"net"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
)

// Listener accepts TCP connections and performs WebSocket handshakes.
type Listener struct {
	ln       net.Listener
	pool     api.BufferPool
	chanCap  int
	numaNode int
}

// NewListener creates a NUMA-aware WebSocket listener.
func NewListener(addr string, pool api.BufferPool, chanCap, numaNode int) (*Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return &Listener{ln: ln, pool: pool, chanCap: chanCap, numaNode: numaNode}, nil
}

// Accept waits for and returns the next WSConnection.
func (l *Listener) Accept() (*protocol.WSConnection, error) {
	conn, err := l.ln.Accept()
	if err != nil {
		return nil, err
	}
	// handshake - use buffered version to preserve any data read after HTTP headers
	hdr, _, br, err := protocol.DoHandshakeCoreBuffered(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake req: %w", err)
	}
	if err := protocol.WriteHandshakeResponse(conn, hdr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake resp: %w", err)
	}
	// wrap transport with buffered reader to not lose any data
	tr := &bufferedConnTransport{conn: conn, br: br, pool: l.pool, numa: l.numaNode}
	ws := protocol.NewWSConnection(tr, l.pool, l.chanCap)
	// Don't call ws.Start() to prevent recvLoop/sendLoop that conflict with server's handleConnWithTracking
	// Server will handle receive operations directly via RecvZeroCopy in handleConnWithTracking
	return ws, nil
}

// Close shuts down the listener.
func (l *Listener) Close() error {
	return l.ln.Close()
}

// bufferedConnTransport implements api.Transport over net.Conn with a bufio.Reader
// to preserve any data buffered during handshake.
type bufferedConnTransport struct {
	conn net.Conn
	br   *bufio.Reader
	pool api.BufferPool
	numa int
}

func (t *bufferedConnTransport) Send(bufs [][]byte) error {
	for _, b := range bufs {
		if _, err := t.conn.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func (t *bufferedConnTransport) Recv() ([][]byte, error) {
	buf := t.pool.Get(4096, t.numa)
	data := buf.Bytes()
	// Read from buffered reader (which wraps conn) to get any buffered data first
	n, err := t.br.Read(data)
	if err != nil {
		buf.Release()
		return nil, err
	}
	return [][]byte{data[:n]}, nil
}

func (t *bufferedConnTransport) Close() error {
	return t.conn.Close()
}

func (t *bufferedConnTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{ZeroCopy: true, Batch: false, NUMAAware: true}
}
