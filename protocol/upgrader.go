// File: protocol/upgrader.go
// Package protocol implements HTTP→WebSocket handshake logic with strict validation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// UpgradeToWebSocket validates the HTTP request headers for a WebSocket upgrade,
// enforces required headers, computes the Sec-WebSocket-Accept key per RFC6455,
// and returns the response headers needed to complete the WebSocket handshake.

package protocol

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

// MaxHandshakeHeadersSize defines the maximum combined length of handshake headers.
const MaxHandshakeHeadersSize = 8192

// UpgradeToWebSocket performs the WebSocket handshake validation and header generation.
// It checks the "Connection" and "Upgrade" headers, reads the "Sec-WebSocket-Key",
// enforces header size limits, computes the accept value per RFC6455,
// and returns the headers to send to the client.
func UpgradeToWebSocket(r *http.Request) (http.Header, error) {
	// Enforce maximum header size to mitigate header injection attacks.
	total := 0
	for k, vs := range r.Header {
		total += len(k)
		for _, v := range vs {
			total += len(v)
		}
		if total > MaxHandshakeHeadersSize {
			return nil, errors.New("handshake headers too large")
		}
	}

	// Validate mandatory upgrade headers, case-insensitive.
	if !headerContainsToken(r.Header, "Connection", "Upgrade") ||
		!headerContainsToken(r.Header, "Upgrade", "websocket") {
		return nil, errors.New("invalid upgrade headers")
	}

	// Extract the Sec-WebSocket-Key from the client request
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key header")
	}

	// Sec-WebSocket-Version must be 13 per RFC6455
	version := r.Header.Get("Sec-WebSocket-Version")
	if version != "13" {
		return nil, errors.New("unsupported WebSocket version; only '13' is supported")
	}

	// Compute accept key
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + guid))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Build the response headers
	resp := make(http.Header)
	resp.Set("Upgrade", "websocket")
	resp.Set("Connection", "Upgrade")
	resp.Set("Sec-WebSocket-Accept", accept)
	// Negotiation of subprotocols and extensions omitted for brevity.

	return resp, nil
}

// headerContainsToken checks if headerName contains the given token, case-insensitive.
func headerContainsToken(h http.Header, headerName, token string) bool {
	vals := h[http.CanonicalHeaderKey(headerName)]
	token = strings.ToLower(token)
	for _, v := range vals {
		parts := strings.Split(v, ",")
		for _, p := range parts {
			if strings.ToLower(strings.TrimSpace(p)) == token {
				return true
			}
		}
	}
	return false
}
