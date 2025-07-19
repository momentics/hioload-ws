// Package protocol
// Author: momentics <momentics@gmail.com>
//
// WebSocket HTTP handshake and upgrade support logic for modern compliant clients.

package protocol

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

// UpgradeToWebSocket handles the HTTP -> WS protocol negotiation.
// Returns the accepted key and constructed response headers.
func UpgradeToWebSocket(r *http.Request) (http.Header, error) {
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

	resp := http.Header{}
	resp.Set("Upgrade", "websocket")
	resp.Set("Connection", "Upgrade")
	resp.Set("Sec-WebSocket-Accept", accept)

	return resp, nil
}
