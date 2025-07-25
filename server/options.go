// File: server/options.go
// Package server defines functional options for the Server facade.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package server

import "github.com/momentics/hioload-ws/api"

// ServerOption customizes server initialization.
type ServerOption func(*Server)

// WithMiddleware attaches middleware in FIFO order.
func WithMiddleware(mw ...Middleware) ServerOption {
	return func(s *Server) {
		s.middleware = append(s.middleware, mw...)
	}
}

// WithAffinityScope sets CPU/NUMA binding scope for reactor and executor.
func WithAffinityScope(scope api.AffinityScope) ServerOption {
	return func(s *Server) {
		s.cfg.AffinityScope = scope
	}
}

// WithBatchSize overrides default reactor batch size.
func WithBatchSize(batch int) ServerOption {
	return func(s *Server) {
		s.cfg.BatchSize = batch
	}
}

// WithExecutorWorkers sets the number of background worker goroutines.
func WithExecutorWorkers(n int) ServerOption {
	return func(s *Server) {
		s.cfg.ExecutorWorkers = n
	}
}
