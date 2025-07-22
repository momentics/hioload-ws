// control/config.go
// Author: momentics <momentics@gmail.com>
//
// Thread-safe configuration store with dynamic update and hot-reload propagation.

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

// NewConfigStore initializes a new config store with empty data.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		config:    make(map[string]any),
		listeners: make([]func(), 0),
	}
}

// GetSnapshot returns a copy of all config values.
func (cs *ConfigStore) GetSnapshot() map[string]any {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	copy := make(map[string]any, len(cs.config))
	for k, v := range cs.config {
		copy[k] = v
	}
	return copy
}

// SetConfig merges new values and dispatches reload if needed.
func (cs *ConfigStore) SetConfig(newCfg map[string]any) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for k, v := range newCfg {
		cs.config[k] = v
	}
	cs.dispatchReload()
}

// OnReload registers a listener hook called on config changes.
func (cs *ConfigStore) OnReload(fn func()) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.listeners = append(cs.listeners, fn)
}

// dispatchReload invokes all listeners.
func (cs *ConfigStore) dispatchReload() {
	for _, fn := range cs.listeners {
		go fn()
	}
}
