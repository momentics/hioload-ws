// internal/websocket/upgrader.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// WebSocket HTTP upgrade handler integrated with hioload-ws transport and buffer pooling.

package websocket

import (
    "crypto/sha1"
    "encoding/base64"
    "errors"
    "net/http"
    "strings"

    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/protocol"
)

// Upgrader bridges HTTP and WSConnection creation.
type Upgrader struct {
    transport  api.Transport
    bufferPool api.BufferPool
}

func NewUpgrader(transport api.Transport, bufferPool api.BufferPool) *Upgrader {
    return &Upgrader{transport: transport, bufferPool: bufferPool}
}

func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*protocol.WSConnection, error) {
    if !strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
        !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
        return nil, errors.New("invalid upgrade headers")
    }
    key := r.Header.Get("Sec-WebSocket-Key")
    if key == "" {
        return nil, errors.New("missing Sec-WebSocket-Key")
    }
    const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
    h := sha1.New()
    h.Write([]byte(key + guid))
    accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
    w.Header().Set("Upgrade", "websocket")
    w.Header().Set("Connection", "Upgrade")
    w.Header().Set("Sec-WebSocket-Accept", accept)
    w.WriteHeader(http.StatusSwitchingProtocols)
    conn := protocol.NewWSConnection(u.transport, u.bufferPool)
    return conn, nil
}

func (u *Upgrader) CheckOrigin(r *http.Request, allowed []string) bool {
    origin := r.Header.Get("Origin")
    if len(allowed) == 0 {
        return true
    }
    for _, o := range allowed {
        if origin == o {
            return true
        }
    }
    return false
}
