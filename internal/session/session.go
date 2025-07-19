// Package session
// Author: momentics <momentics@gmail.com>
//
// Core interfaces and types for connection/session state handling.
// Stateless transport connections (like WebSocket) wrap a session object with user scope.

package session

import (
	"time"
	"github.com/momentics/hioload-ws/api"
)

// Session represents a long-lived user or transport connection context.
type Session interface {
	// ID returns global unique session key.
	ID() string

	// Context returns the scoped context storage.
	Context() api.Context

	// Cancel cancels the session (lifecycle end).
	Cancel()

	// Done signals session closure.
	Done() <-chan struct{}

	// Deadline returns expiry time if set.
	Deadline() (time.Time, bool)
}

// SessionManager manages lifetime, lookup, and storage of active session objects.
type SessionManager interface {
	// Create inserts a session for a given ID, if not exists.
	Create(id string) (Session, error)

	// Get fetches a session by ID.
	Get(id string) (Session, bool)

	// Delete closes and removes a session.
	Delete(id string)

	// Range iterates all active sessions.
	Range(fn func(Session))
}
