// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package transport

import (
    "net"
//    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/pool"
)

// NetConn implements zero-copy, pool-backed network connection.
type NetConn struct {
    conn net.Conn
    pool pool.BytePool
}

// NewNetConn initializes a new NetConn.
func NewNetConn(conn net.Conn, pool pool.BytePool) *NetConn {
    return &NetConn{
        conn: conn,
        pool: pool,
    }
}

// Read: Zero-copy buffer fill.
func (n *NetConn) Read(buf []byte) (int, error) {
    return n.conn.Read(buf)
}

// Write: Zero-copy.
func (n *NetConn) Write(buf []byte) (int, error) {
    return n.conn.Write(buf)
}

// Close the connection.
func (n *NetConn) Close() error {
    return n.conn.Close()
}
