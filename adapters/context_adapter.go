// File: adapters/context_adapter.go
package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/session"
)

// ContextAdapter implements api.ContextFactory by producing new context stores.
type ContextAdapter struct{}

// NewContextAdapter returns an instance of the context factory.
func NewContextAdapter() api.ContextFactory {
	return &ContextAdapter{}
}

// NewContext returns a new Context (backed by internal/session contextStore).
func (a *ContextAdapter) NewContext() api.Context {
	return session.NewContextStore()
}
