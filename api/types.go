// File: api/types.go
// Author: momentics <momentics@gmail.com>
//
// Shared API-level type declarations, DTOs, and constants.

package api

import "time"

// SessionStatus enumerates the state of a WebSocket session.
type SessionStatus int

const (
	SessionUnknown SessionStatus = iota
	SessionConnecting
	SessionActive
	SessionClosing
	SessionClosed
)

func (s SessionStatus) String() string {
	switch s {
	case SessionConnecting:
		return "connecting"
	case SessionActive:
		return "active"
	case SessionClosing:
		return "closing"
	case SessionClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// APIMetrics provides a standard layout for service health/statistics reporting.
type APIMetrics struct {
	NumSessions     int
	NumMessages     int
	InboundTraffic  uint64 // bytes received
	OutboundTraffic uint64 // bytes sent
	StartedAt       time.Time
}

// ServiceInfo exposes descriptive build- and runtime info for external tools.
type ServiceInfo struct {
	Name      string
	Version   string
	Build     string
	StartedAt time.Time
}
