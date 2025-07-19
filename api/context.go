// Package api
// Author: momentics@gmail.com
//
// Extensible, typed execution context.

package api

// Context is a key-value, type-safe execution context.
type Context interface {
    // Set sets a value with given key.
    Set(key string, value any)
    // Get retrieves a value by key.
    Get(key string) (any, bool)
    // Delete removes a key.
    Delete(key string)
    // Clone creates a shallow copy.
    Clone() Context
}
