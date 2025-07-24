// control/config.go
// Thread-safe configuration store with dynamic update and hot-reload propagation.
// This version introduces SetConfigSync for synchronous listener notification.

package control

import (
	"sync"
)

// ConfigStore is a dynamic key/value map with atomic snapshot and listener support.
type ConfigStore struct {
	mu        sync.RWMutex
	config    map[string]any
	listeners []func()
}

// NewConfigStore initializes a new config store.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		config:    make(map[string]any),
		listeners: make([]func(), 0),
	}
}

// GetSnapshot returns a copy of all config entries.
func (cs *ConfigStore) GetSnapshot() map[string]any {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	copy := make(map[string]any, len(cs.config))
	for k, v := range cs.config {
		copy[k] = v
	}
	return copy
}

// SetConfig merges new values and dispatches reload asynchronously (for production use).
func (cs *ConfigStore) SetConfig(newCfg map[string]any) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for k, v := range newCfg {
		cs.config[k] = v
	}
	cs.dispatchReload()
}

// SetConfigSync merges new values and invokes all listeners synchronously (useful for tests).
func (cs *ConfigStore) SetConfigSync(newCfg map[string]any) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for k, v := range newCfg {
		cs.config[k] = v
	}
	// Synchronously invoke each listener in the same goroutine.
	for _, fn := range cs.listeners {
		fn()
	}
}

// OnReload registers a new reload callback.
func (cs *ConfigStore) OnReload(fn func()) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.listeners = append(cs.listeners, fn)
}

// dispatchReload invokes all listeners asynchronously (default for production).
func (cs *ConfigStore) dispatchReload() {
	for _, fn := range cs.listeners {
		go fn()
	}
}
