// File: protocol/native_handshake.go
// Package protocol provides native WebSocket handshake without HTTP dependency.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This implements RFC6455 WebSocket handshake directly, bypassing net/http
// for maximum performance in high-load scenarios.

package protocol

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"strings"
)

// HTTP-заголовки для WebSocket upgrade.
const (
	HeaderConnection      = "connection"
	HeaderUpgrade         = "upgrade"
	HeaderSecWebSocketKey = "sec-websocket-key"
)

// Поддерживаемые значения заголовков.
const (
	ValueUpgrade   = "upgrade"
	ValueWebSocket = "websocket"
	WebSocketGUID  = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// Ошибки валидации HTTP-заголовков WebSocket upgrade.
var (
	ErrInvalidUpgradeHeaders = errors.New("invalid WebSocket upgrade headers")
	ErrMissingWebSocketKey   = errors.New("missing Sec-WebSocket-Key header")
)

// ComputeAcceptKey computes the Sec-WebSocket-Accept value from the client's key.
// This implements the algorithm specified in RFC6455 Section 1.3.
func ComputeAcceptKey(clientKey string) string {
	combined := clientKey + WebSocketGUID
	hash := sha1.Sum([]byte(combined))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// ValidateUpgradeHeaders checks if the connection headers are valid for WebSocket upgrade.
func ValidateUpgradeHeaders(headers map[string]string) error {
	connection := headers[HeaderConnection]
	upgrade := headers[HeaderUpgrade]
	wsKey := headers[HeaderSecWebSocketKey]

	if connection == "" || upgrade == "" {
		return ErrInvalidUpgradeHeaders
	}

	// Проверка token-ов (case-insensitive)
	if !containsToken(connection, ValueUpgrade) {
		return ErrInvalidUpgradeHeaders
	}
	if !containsToken(upgrade, ValueWebSocket) {
		return ErrInvalidUpgradeHeaders
	}
	if wsKey == "" {
		return ErrMissingWebSocketKey
	}
	return nil
}

// containsToken checks if value содержит токен (case-insensitive).
func containsToken(headerValue, token string) bool {
	parts := strings.Split(headerValue, ",")
	token = strings.ToLower(strings.TrimSpace(token))
	for _, p := range parts {
		if strings.ToLower(strings.TrimSpace(p)) == token {
			return true
		}
	}
	return false
}
