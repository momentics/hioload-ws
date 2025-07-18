// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package protocol

import (
    "github.com/momentics/hioload-ws/api"
//    "encoding/binary"
)

// WebSocketFrame: Implements simple non-validating frame parser.
type WebSocketFrame struct {
    Header  []byte
    Payload []byte
    PoolRef api.BytePool
}

// NewWebSocketFrame allocates a new frame from pool.
func NewWebSocketFrame(pool api.BytePool, payload []byte) *WebSocketFrame {
    // Minimal header size 2
    frame := &WebSocketFrame{
        Header:  make([]byte, 2),
        Payload: payload,
        PoolRef: pool,
    }
    frame.Header[0] = 0x81     // FIN + text opcode
    frame.Header[1] = byte(len(payload))
    return frame
}

// Release: return payload to originating pool.
func (f *WebSocketFrame) Release() {
    if f.PoolRef != nil && f.Payload != nil {
        f.PoolRef.Put(f.Payload)
        f.Payload = nil
    }
}
