// File: protocol/handshake_serializer.go
// Package protocol
// Helper functions для сериализации HTTP-заголовков WebSocket-handshake.
package protocol

import (
	"fmt"
	"io"
	"net/http"
)

// WriteHandshakeResponse записывает в w статус 101 и заголовки hdr.
func WriteHandshakeResponse(w io.Writer, hdr http.Header) error {
	// Статус и служебные заголовки уже в hdr должны содержаться
	if _, err := fmt.Fprintf(w, "HTTP/1.1 101 Switching Protocols\r\n"); err != nil {
		return err
	}
	for k, vs := range hdr {
		for _, v := range vs {
			if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprint(w, "\r\n"); err != nil {
		return err
	}
	return nil
}
