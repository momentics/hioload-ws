// File: adapters/handler_adapter.go
// Package adapters
// Author: momentics <momentics@gmail.com>
//
// HandlerFunc glue and extensible middleware with chain-of-type tracing.

package adapters

import (
	"log"

	"github.com/momentics/hioload-ws/api"
)

// HandlerFunc converts a function into an api.Handler.
type HandlerFunc func(data any) error

// Handle calls the underlying function.
func (f HandlerFunc) Handle(data any) error {
	return f(data)
}

// MiddlewareHandler wraps a base Handler and applies middleware in chain.
type MiddlewareHandler struct {
	handler    api.Handler
	middleware []func(api.Handler) api.Handler
}

// NewMiddlewareHandler creates a new MiddlewareHandler for the given base handler.
func NewMiddlewareHandler(handler api.Handler) *MiddlewareHandler {
	return &MiddlewareHandler{
		handler:    handler,
		middleware: make([]func(api.Handler) api.Handler, 0),
	}
}

// Use appends a middleware to the chain.
func (m *MiddlewareHandler) Use(mw func(api.Handler) api.Handler) *MiddlewareHandler {
	m.middleware = append(m.middleware, mw)
	return m
}

// Handle applies all middleware then calls the base handler.
func (m *MiddlewareHandler) Handle(data any) error {
	handler := m.handler
	for i := len(m.middleware) - 1; i >= 0; i-- {
		handler = m.middleware[i](handler)
	}
	return handler.Handle(data)
}

// LoggingMiddleware logs entry, exit, and errors of handler invocation.
func LoggingMiddleware(next api.Handler) api.Handler {
	return HandlerFunc(func(data any) error {
		log.Printf("[Handler] Processing data: %T", data)
		err := next.Handle(data)
		if err != nil {
			log.Printf("[Handler] Error: %v", err)
		}
		return err
	})
}

// RecoveryMiddleware recovers from panics in handler.
func RecoveryMiddleware(next api.Handler) api.Handler {
	return HandlerFunc(func(data any) error {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Handler] Panic recovered: %v", r)
			}
		}()
		return next.Handle(data)
	})
}

// MetricsMiddleware increments the "handler.processed" counter in Control stats.
func MetricsMiddleware(control api.Control) func(api.Handler) api.Handler {
	return func(next api.Handler) api.Handler {
		return HandlerFunc(func(data any) error {
			stats := control.Stats()
			count, _ := stats["handler.processed"].(int64)
			control.SetConfig(map[string]any{
				"handler.processed": count + 1,
			})
			return next.Handle(data)
		})
	}
}
