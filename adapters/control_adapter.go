// File: adapters/control_adapter.go
// Package adapters implements the api.Control interface using control package primitives.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This version ensures reload hooks in SetConfig are called synchronously so tests
// reliably observe OnReload before SetConfig returns.

package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/control"
)

// ControlAdapter bridges api.Control to internal control primitives.
// It merges config, exposes stats, and enables hot-reload hooks.
type ControlAdapter struct {
	config  *control.ConfigStore
	metrics *control.MetricsRegistry
	debug   *control.DebugProbes
}

// NewControlAdapter constructs a new adapter that provides all Control features.
func NewControlAdapter() api.Control {
	adapter := &ControlAdapter{
		config:  control.NewConfigStore(),
		metrics: control.NewMetricsRegistry(),
		debug:   control.NewDebugProbes(),
	}
	// Register platform-specific debug probes.
	control.RegisterPlatformProbes(adapter.debug)
	return adapter
}

// GetConfig returns a snapshot of the current config state.
func (c *ControlAdapter) GetConfig() map[string]any {
	return c.config.GetSnapshot()
}

// SetConfig synchronously updates configuration and invokes all listeners and reload hooks.
// This solves test flakiness by making OnReload deterministic.
func (c *ControlAdapter) SetConfig(cfg map[string]any) error {
	// 1. Merge new values and synchronously notify instance listeners.
	c.config.SetConfigSync(cfg)
	// 2. Synchronously invoke all global hot-reload hooks for test determinism.
	control.TriggerHotReloadSync()
	return nil
}

// Stats returns a merged map of config, metrics, and debug-probe data.
func (c *ControlAdapter) Stats() map[string]any {
	combined := make(map[string]any)
	for k, v := range c.config.GetSnapshot() {
		combined[k] = v
	}
	for k, v := range c.metrics.GetSnapshot() {
		combined["metrics."+k] = v
	}
	for k, v := range c.debug.DumpState() {
		combined["debug."+k] = v
	}
	return combined
}

// OnReload registers a new hot-reload callback.
// Both instance and global registration are used for completeness.
func (c *ControlAdapter) OnReload(fn func()) {
	c.config.OnReload(fn)
	control.RegisterReloadHook(fn)
}

// RegisterDebugProbe allows attaching custom debug probes for diagnostics.
func (c *ControlAdapter) RegisterDebugProbe(name string, fn func() any) {
	c.debug.RegisterProbe(name, fn)
}

// GetDebug provides access to the debug probe subsystem.
func (c *ControlAdapter) GetDebug() api.Debug {
	return c.debug
}
