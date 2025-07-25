// File: api/context.go
// Package api
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Strongly typed, extensible context contract with explicit key scoping and
// cross-layer propagation control for high-performance workflows.
// Not compatible with standard context.Context.

package api

import (
	"context"
)

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

type contextKey string

const connectionKey contextKey = "hioload-ws-connection"

// ContextWithConnection returns a new context containing the given connection.
func ContextWithConnection(ctx context.Context, conn any) context.Context {
	return context.WithValue(ctx, connectionKey, conn)
}

// ContextFromData extracts the context from handler data.
// It expects data to wrap a context, for example via ContextualHandler.
func ContextFromData(data any) context.Context {
	// If data implements interface{ Ctx() context.Context }, use that.
	if withCtx, ok := data.(interface{ Ctx() context.Context }); ok {
		return withCtx.Ctx()
	}
	// Fallback to background context.
	return context.Background()
}

// FromContext retrieves the connection stored in context.
func FromContext(ctx context.Context) any {
	return ctx.Value(connectionKey)
}
