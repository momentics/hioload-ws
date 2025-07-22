// File: internal/session/context_store.go
// Package session
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Thread-safe, propagation-aware context store implementing api.Context.

package session

import (
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// entry holds value, propagation flag и expiry timestamp.
type entry struct {
	value      any
	propagated bool
	expiry     time.Time
}

// contextStore holds key/value entries with optional TTL and propagation.
type contextStore struct {
	mu    sync.RWMutex
	store map[string]entry
}

// Ensure compile-time API compliance.
var _ api.Context = (*contextStore)(nil)

// NewContextStore создаёт новый internal/session.contextStore.
func NewContextStore() *contextStore {
	return &contextStore{store: make(map[string]entry)}
}

// Set assigns a value under key, marking it for propagation if requested.
func (c *contextStore) Set(key string, value any, propagated bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = entry{value: value, propagated: propagated}
}

// Get retrieves a value by key; returns false if missing or expired.
func (c *contextStore) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[key]
	if !ok {
		return nil, false
	}
	if !e.expiry.IsZero() && time.Now().After(e.expiry) {
		return nil, false
	}
	return e.value, true
}

// Delete removes the key from the store.
func (c *contextStore) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

// Clone produces a shallow copy of the contextStore for propagation.
func (c *contextStore) Clone() api.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copyMap := make(map[string]entry, len(c.store))
	for k, v := range c.store {
		copyMap[k] = v
	}
	return &contextStore{store: copyMap}
}

// WithExpiration sets a TTL (in nanoseconds) on the given key.
func (c *contextStore) WithExpiration(key string, ttlNanos int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.store[key]; ok {
		e.expiry = time.Now().Add(time.Duration(ttlNanos))
		c.store[key] = e
	}
}

// IsPropagated returns whether the key is marked for propagation.
func (c *contextStore) IsPropagated(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[key]
	return ok && e.propagated
}

// Keys returns all non-expired keys in the store.
func (c *contextStore) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := time.Now()
	keys := make([]string, 0, len(c.store))
	for k, v := range c.store {
		if v.expiry.IsZero() || v.expiry.After(now) {
			keys = append(keys, k)
		}
	}
	return keys
}
