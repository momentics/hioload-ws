// File: internal/websocket/upgrader.go
// HTTP→WebSocket upgrade and connection creation.
package websocket

import (
    "net/http"
    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/protocol"
)

// Upgrader handles HTTP→WebSocket upgrade and returns WSConnection.
type Upgrader struct {
    transport   api.Transport
    bufPool     api.BufferPool
    channelSize int
}

func NewUpgrader(tr api.Transport, bp api.BufferPool, chSize int) *Upgrader {
    return &Upgrader{transport: tr, bufPool: bp, channelSize: chSize}
}

func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*protocol.WSConnection, error) {
    // Delegate to core handshake + serializer
    hdrs, err := protocol.DoHandshakeCore(r.Body)
    if err != nil {
        return nil, err
    }
    for k, vs := range hdrs {
        for _, v := range vs {
            w.Header().Add(k, v)
        }
    }
    w.WriteHeader(http.StatusSwitchingProtocols)
    conn := protocol.NewWSConnection(u.transport, u.bufPool, u.channelSize)
    return conn, nil
}
