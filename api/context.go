// File: api/context.go
// Package api
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Strongly typed, extensible context contract with explicit key scoping and
// cross-layer propagation control for high-performance workflows.
// Not compatible with standard context.Context.

package api

// Context provides a lightweight key-value store with explicit propagation semantics.
type Context interface {
	// Set assigns a value for a key, optionally marking it as propagated.
	Set(key string, value any, propagated bool)
	// Get fetches a value, returning (value, exists).
	Get(key string) (any, bool)
	// Delete removes a value/key.
	Delete(key string)
	// Clone returns a shallow copy of the context suitable for child operations.
	Clone() Context
	// WithExpiration sets a TTL for key(s), notifying on expiry.
	WithExpiration(key string, ttlNanos int64)
	// IsPropagated checks if a key is marked for propagation.
	IsPropagated(key string) bool
	// Keys returns all present keys.
	Keys() []string
}
