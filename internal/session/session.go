// File: internal/session/session.go
// Package session
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Core session implementation with cancellation, deadline, and context.

package session

import (
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// sessionImpl holds per-connection state, context, and cancellation.
type sessionImpl struct {
	id       string
	ctx      api.Context
	done     chan struct{}
	once     sync.Once
	deadline time.Time
}

// Ensure compile-time API compliance if api.Session exists:
// var _ api.Session = (*sessionImpl)(nil)

// newSession creates a new session with the given unique identifier.
func newSession(id string) *sessionImpl {
	return &sessionImpl{
		id:   id,
		ctx:  NewContextStore(),
		done: make(chan struct{}),
	}
}

// ID returns the unique session identifier.
func (s *sessionImpl) ID() string {
	return s.id
}

// Context returns the underlying api.Context.
func (s *sessionImpl) Context() api.Context {
	return s.ctx
}

// Cancel signals session teardown; idempotent.
func (s *sessionImpl) Cancel() {
	s.once.Do(func() {
		close(s.done)
	})
}

// Done returns a channel closed upon cancellation.
func (s *sessionImpl) Done() <-chan struct{} {
	return s.done
}

// Deadline returns the session expiration if set.
func (s *sessionImpl) Deadline() (time.Time, bool) {
	if s.deadline.IsZero() {
		return time.Time{}, false
	}
	return s.deadline, true
}

// WithDeadline sets an absolute deadline for the session.
func (s *sessionImpl) WithDeadline(t time.Time) {
	s.deadline = t
}
