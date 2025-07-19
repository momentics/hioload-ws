// Package session
// Author: momentics <momentics@gmail.com>
//
// Thread-safe, propagation-aware high-performance context store for sessions.

package session

import (
	"sync"
	"time"
)

type entry struct {
	val        any
	propagated bool
	expiry     time.Time
}

type contextStore struct {
	mu    sync.RWMutex
	store map[string]entry
}

// newContextStore creates an empty, thread-safe context store.
func newContextStore() *contextStore {
	return &contextStore{
		store: make(map[string]entry),
	}
}

// Set stores a key-value pair with optional propagation flag.
func (c *contextStore) Set(key string, value any, propagated bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = entry{val: value, propagated: propagated}
}

// Get retrieves a value and its existence.
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

// Clone returns a shallow clone of the context.
func (c *contextStore) Clone() *contextStore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cp := make(map[string]entry, len(c.store))
	for k, v := range c.store {
		cp[k] = v
	}
	return &contextStore{
		store: cp,
	}
}

// WithExpiration sets expiration timestamp for a key.
func (c *contextStore) WithExpiration(key string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.store[key]; ok {
		e.expiry = time.Now().Add(ttl)
		c.store[key] = e
	}
}

// Keys returns all active keys.
func (c *contextStore) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.store))
	for k := range c.store {
		keys = append(keys, k)
	}
	return keys
}

// IsPropagated reports propagation flag.
func (c *contextStore) IsPropagated(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.store[key]
	return ok && e.propagated
}
