// File: adapters/control_adapter.go
// Package adapters
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Control adapter implementing api.Control interface using control package primitives.

package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/control"
)

// ControlAdapter bridges api.Control to internal control primitives.
type ControlAdapter struct {
	config  *control.ConfigStore
	metrics *control.MetricsRegistry
	debug   *control.DebugProbes
}

// NewControlAdapter constructs a new ControlAdapter.
func NewControlAdapter() api.Control {
	adapter := &ControlAdapter{
		config:  control.NewConfigStore(),
		metrics: control.NewMetricsRegistry(),
		debug:   control.NewDebugProbes(),
	}
	// Register platform-specific probes
	control.RegisterPlatformProbes(adapter.debug)
	return adapter
}

// GetConfig returns a snapshot of the current configuration.
func (c *ControlAdapter) GetConfig() map[string]any {
	return c.config.GetSnapshot()
}

// SetConfig merges and applies new configuration, then triggers reload hooks.
func (c *ControlAdapter) SetConfig(cfg map[string]any) error {
	// Update instance config store
	c.config.SetConfig(cfg)
	// Trigger any registered reload hooks (both instance and global)
	control.TriggerHotReload()
	return nil
}

// Stats returns merged config snapshot, metrics and debug probe data.
func (c *ControlAdapter) Stats() map[string]any {
	// Start with current config values
	combined := make(map[string]any)
	for k, v := range c.config.GetSnapshot() {
		combined[k] = v
	}
	// Merge metrics snapshot
	for k, v := range c.metrics.GetSnapshot() {
		combined["metrics."+k] = v
	}
	// Merge debug probes state
	for k, v := range c.debug.DumpState() {
		combined["debug."+k] = v
	}
	return combined
}

// OnReload registers a callback invoked on configuration changes.
func (c *ControlAdapter) OnReload(fn func()) {
	c.config.OnReload(fn)
	control.RegisterReloadHook(fn)
}

// RegisterDebugProbe registers a named debug probe function.
func (c *ControlAdapter) RegisterDebugProbe(name string, fn func() any) {
	c.debug.RegisterProbe(name, fn)
}

// File: adapters/control_adapter.go
func (c *ControlAdapter) GetDebug() api.Debug {
	return c.debug
}
