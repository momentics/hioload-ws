// File: adapters/context_adapter.go
package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/session"
)

// ContextAdapter реализует api.ContextFactory, создавая internal/session.contextStore.
type ContextAdapter struct{}

// NewContextAdapter возвращает фабрику контекстов.
func NewContextAdapter() api.ContextFactory {
	return &ContextAdapter{}
}

// NewContext создаёт новый бесплатный contextStore.
func (a *ContextAdapter) NewContext() api.Context {
	return session.NewContextStore()
}
