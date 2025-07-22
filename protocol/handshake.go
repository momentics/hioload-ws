// File: protocol/handshake.go
// Package protocol
// Core logic WebSocket handshake: парсинг HTTP-запроса, валидация заголовков,
// вычисление Sec-WebSocket-Accept. Возвращает готовые заголовки ответа.
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

const (
	WebSocketGUID            = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	MaxHandshakeHeadersSize  = 8192
	HeaderConnection         = "Connection"
	HeaderUpgrade            = "Upgrade"
	HeaderSecWebSocketKey    = "Sec-WebSocket-Key"
	HeaderSecWebSocketVer    = "Sec-WebSocket-Version"
	RequiredWebSocketVersion = "13"
)

var (
	ErrInvalidUpgradeHeaders = fmt.Errorf("invalid WebSocket upgrade headers")
	ErrMissingWebSocketKey   = fmt.Errorf("missing Sec-WebSocket-Key header")
	ErrBadWebSocketVersion   = fmt.Errorf("unsupported WebSocket version; only '13' is supported")
)

// DoHandshakeCore читает HTTP-запрос из r, валидирует необходимые заголовки,
// вычисляет Sec-WebSocket-Accept и возвращает http.Header для ответа.
func DoHandshakeCore(r io.Reader) (http.Header, error) {
	br := bufio.NewReader(r)
	req, err := http.ReadRequest(br)
	if err != nil {
		return nil, fmt.Errorf("handshake read request: %w", err)
	}
	// Ограничение размера заголовков
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
	// Проверка Connection и Upgrade
	if !headerContainsToken(req.Header, HeaderConnection, "Upgrade") ||
		!headerContainsToken(req.Header, HeaderUpgrade, "websocket") {
		return nil, ErrInvalidUpgradeHeaders
	}
	// Проверка версии
	if req.Header.Get(HeaderSecWebSocketVer) != RequiredWebSocketVersion {
		return nil, ErrBadWebSocketVersion
	}
	// Ключ клиента
	key := req.Header.Get(HeaderSecWebSocketKey)
	if key == "" {
		return nil, ErrMissingWebSocketKey
	}
	// Вычисление ответа
	h := sha1.New()
	h.Write([]byte(key + WebSocketGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	// Формирование заголовков ответа
	hdr := make(http.Header)
	hdr.Set("Upgrade", "websocket")
	hdr.Set("Connection", "Upgrade")
	hdr.Set("Sec-WebSocket-Accept", accept)
	return hdr, nil
}

// headerContainsToken проверяет наличие токена token в headerName.
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
