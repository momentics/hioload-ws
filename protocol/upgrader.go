// File: protocol/upgrader.go
// Package protocol implements HTTP→WebSocket handshake logic.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// UpgradeToWebSocket validates the HTTP request headers for a WebSocket upgrade,
// computes the Sec-WebSocket-Accept key, and returns the response headers
// needed to complete the WebSocket handshake.

package protocol

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

// UpgradeToWebSocket performs the WebSocket handshake validation and header generation.
// It checks the "Connection" and "Upgrade" headers, reads the "Sec-WebSocket-Key",
// computes the accept value per RFC6455, and returns the headers to send to the client.
func UpgradeToWebSocket(r *http.Request) (http.Header, error) {
	// Validate mandatory upgrade headers
	if !strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
		!strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("invalid upgrade headers")
	}

	// Extract the Sec-WebSocket-Key from the client request
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}

	// Concatenate with GUID and compute SHA-1 hash
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + guid))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Build the response headers
	resp := make(http.Header)
	resp.Set("Upgrade", "websocket")
	resp.Set("Connection", "Upgrade")
	resp.Set("Sec-WebSocket-Accept", accept)
	return resp, nil
}
