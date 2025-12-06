// File: protocol/handshake.go
// Package protocol implements the core WebSocket handshake logic for hioload-ws.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Provides both server-side and client-side handshake routines to ensure
// a single, consistent implementation of RFC6455 HTTP Upgrade processing,
// Sec-WebSocket-Key/Accept negotiation, and header serialization.

package protocol

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Constants used for handshake processing.
const (
	WebSocketGUID            = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	HeaderConnection         = "Connection"
	HeaderUpgrade            = "Upgrade"
	HeaderSecWebSocketKey    = "Sec-WebSocket-Key"
	HeaderSecWebSocketVer    = "Sec-WebSocket-Version"
	RequiredWebSocketVersion = "13"
	MaxHandshakeHeadersSize  = 8192
)

// Errors for handshake validation.
var (
	ErrInvalidUpgradeHeaders = fmt.Errorf("invalid WebSocket upgrade headers")
	ErrMissingWebSocketKey   = fmt.Errorf("missing Sec-WebSocket-Key header")
	ErrBadWebSocketVersion   = fmt.Errorf("unsupported WebSocket version; only '13' is supported")
)

// DoHandshakeCore reads and validates the HTTP/1.1 Upgrade request from r.
// Returns the headers to include in the HTTP 101 Switching Protocols response.
func DoHandshakeCore(r io.Reader) (http.Header, error) {

	
	br := bufio.NewReader(r)
	req, err := http.ReadRequest(br)
	if err != nil {
		return nil, fmt.Errorf("handshake read request: %w", err)
	}

	// Enforce a maximum total header size to prevent abuse.
	total := 0
	for k, vs := range req.Header {
		total += len(k)
		for _, v := range vs {
			total += len(v)
			if total > MaxHandshakeHeadersSize {
				return nil, fmt.Errorf("handshake headers too large")
			}
		}
	}

	// Validate required upgrade tokens.
	if !headerContainsToken(req.Header, HeaderConnection, "Upgrade") ||
		!headerContainsToken(req.Header, HeaderUpgrade, "websocket") {
		return nil, ErrInvalidUpgradeHeaders
	}

	// Verify WebSocket version.
	if req.Header.Get(HeaderSecWebSocketVer) != RequiredWebSocketVersion {
		return nil, ErrBadWebSocketVersion
	}

	// Extract client key.
	key := req.Header.Get(HeaderSecWebSocketKey)
	if key == "" {
		return nil, ErrMissingWebSocketKey
	}

	// Compute the Sec-WebSocket-Accept.
	h := sha1.New()
	h.Write([]byte(key + WebSocketGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Prepare response headers.
	hdr := make(http.Header)
	hdr.Set("Upgrade", "websocket")
	hdr.Set("Connection", "Upgrade")
	hdr.Set("Sec-WebSocket-Accept", accept)
	return hdr, nil
}

// WriteHandshakeResponse writes the HTTP/1.1 101 Switching Protocols response
// with the provided headers to w. Caller must include required headers.
func WriteHandshakeResponse(w io.Writer, hdr http.Header) error {
	// Status line.
	
	if _, err := fmt.Fprintf(w, "HTTP/1.1 101 Switching Protocols\r\n"); err != nil {
		return err
	}
	// Write all headers.
	for k, vs := range hdr {
		for _, v := range vs {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	// End of headers.
	if _, err := fmt.Fprint(w, "\r\n"); err != nil {
		return err
	}
	return nil
}

// WriteHandshakeRequest serializes the HTTP GET Upgrade request into w,
// using the provided http.Request. Ensures only the request-line path is used.
func WriteHandshakeRequest(w io.Writer, req *http.Request) error {
	// Force the RequestURI to be just the path.
	req.RequestURI = ""
	// Write request-line and headers.
	if err := req.Write(w); err != nil {
		return fmt.Errorf("handshake write request: %w", err)
	}
	return nil
}

// DoClientHandshake reads and validates the HTTP/1.1 101 Switching Protocols response
// from r, using the original req for correct parsing context. Discards remaining buffer.
func DoClientHandshake(r io.Reader, req *http.Request) error {
	br := bufio.NewReader(r)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return fmt.Errorf("handshake read response: %w", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return fmt.Errorf("handshake failed: status %d", resp.StatusCode)
	}
	// Discard any buffered data remaining after headers.
	io.Copy(io.Discard, br)
	return nil
}

// headerContainsToken checks if headerName contains the given token (case-insensitive).
func headerContainsToken(h http.Header, headerName, token string) bool {
	vals := h[http.CanonicalHeaderKey(headerName)]
	token = strings.ToLower(token)
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			if strings.ToLower(strings.TrimSpace(part)) == token {
				return true
			}
		}
	}
	return false
}
