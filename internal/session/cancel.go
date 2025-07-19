// Package session
// Author: momentics <momentics@gmail.com>
//
// Session lifecycle management including cancellation and context propagation.
// This component implements high-throughput, thread-safe sessions with scoped context,
// cancellation signals, and cloneable propagation-aware key/value storage.

package session

import (
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
)

type entry struct {
	val        any
	propagated bool
	expiry     time.Time
}

// contextStore is a thread-safe, cloneable implementation of api.Context.
type contextStore struct {
	mu    sync.RWMutex
	store map[string]entry
}

// Ensure compliance with api.Context interface.
var _ api.Context = (*contextStore)(nil)

func newContextStore() *contextStore {
	return &contextStore{
		store: make(map[string]entry),
	}
}

// Set assigns a value with optional propagation.
func (c *contextStore) Set(key string, value any, propagated bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = entry{val: value, propagated: propagated}
}

// Get fetches a value, returning (value, exists).
func (c *contextStore) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[key]
	if !ok || (!e.expiry.IsZero() && time.Now().After(e.expiry)) {
		return nil, false
	}
	return e.val, true
}

// Delete removes a key.
func (c *contextStore) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

// Clone creates a shallow copy satisfying api.Context.
func (c *contextStore) Clone() api.Context {
	cp := make(map[string]entry, len(c.store))
	c.mu.RLock()
	for k, v := range c.store {
		cp[k] = v
	}
	c.mu.RUnlock()
	return &contextStore{store: cp}
}

// WithExpiration sets a TTL for a specific key.
func (c *contextStore) WithExpiration(key string, ttlNanos int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.store[key]; ok {
		e.expiry = time.Now().Add(time.Duration(ttlNanos))
		c.store[key] = e
	}
}

// IsPropagated checks if a key is marked for propagation.
func (c *contextStore) IsPropagated(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[key]
	return ok && e.propagated
}

// Keys returns all active keys in the context.
func (c *contextStore) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.store))
	for k := range c.store {
		keys = append(keys, k)
	}
	return keys
}

// sessionImpl represents a single high-level session instance.
type sessionImpl struct {
	id       string
	ctx      *contextStore
	done     chan struct{}
	once     sync.Once
	deadline time.Time
}

// newSession constructs a new session with the given ID.
func newSession(id string) *sessionImpl {
	return &sessionImpl{
		id:   id,
		ctx:  newContextStore(),
		done: make(chan struct{}),
	}
}

// ID returns the session identifier.
func (s *sessionImpl) ID() string {
	return s.id
}

// Context exposes session context.
func (s *sessionImpl) Context() api.Context {
	return s.ctx
}

// Cancel triggers session teardown and closure.
func (s *sessionImpl) Cancel() {
	s.once.Do(func() {
		close(s.done)
	})
}

// Done returns closed chan on session termination.
func (s *sessionImpl) Done() <-chan struct{} {
	return s.done
}

// Deadline returns an optional session timeout hint.
func (s *sessionImpl) Deadline() (time.Time, bool) {
	if s.deadline.IsZero() {
		return time.Time{}, false
	}
	return s.deadline, true
}
