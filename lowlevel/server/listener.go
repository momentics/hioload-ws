package server

import (
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
	// handshake
	hdr, err := protocol.DoHandshakeCore(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake req: %w", err)
	}
	if err := protocol.WriteHandshakeResponse(conn, hdr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake resp: %w", err)
	}
	// wrap transport
	tr := &connTransport{conn: conn, pool: l.pool, numa: l.numaNode}
	ws := protocol.NewWSConnection(tr, l.pool, l.chanCap)
	ws.Start()
	return ws, nil
}

// Close shuts down the listener.
func (l *Listener) Close() error {
	return l.ln.Close()
}

// connTransport implements api.Transport over net.Conn.
type connTransport struct {
	conn net.Conn
	pool api.BufferPool
	numa int
}

func (t *connTransport) Send(bufs [][]byte) error {
	for _, b := range bufs {
		if _, err := t.conn.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func (t *connTransport) Recv() ([][]byte, error) {
	buf := t.pool.Get(4096, t.numa)
	data := buf.Bytes()
	n, err := t.conn.Read(data)
	if err != nil {
		buf.Release()
		return nil, err
	}
	return [][]byte{data[:n]}, nil
}

func (t *connTransport) Close() error {
	return t.conn.Close()
}

func (t *connTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{ZeroCopy: true, Batch: false, NUMAAware: true}
}
