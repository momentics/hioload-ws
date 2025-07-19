// Package adapters
// Author: momentics <momentics@gmail.com>
//
// Control adapter implementing api.Control interface using control package primitives.

package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/control"
)

type ControlAdapter struct {
	config  *control.ConfigStore
	metrics *control.MetricsRegistry
	debug   *control.DebugProbes
}

func NewControlAdapter() api.Control {
	adapter := &ControlAdapter{
		config:  control.NewConfigStore(),
		metrics: control.NewMetricsRegistry(),
		debug:   control.NewDebugProbes(),
	}
	control.RegisterPlatformProbes(adapter.debug)
	return adapter
}

func (c *ControlAdapter) GetConfig() map[string]any {
	return c.config.GetSnapshot()
}
func (c *ControlAdapter) SetConfig(cfg map[string]any) error {
	c.config.SetConfig(cfg)
	return nil
}
func (c *ControlAdapter) Stats() map[string]any {
	stats := c.metrics.GetSnapshot()
	debugStats := c.debug.DumpState()
	combined := make(map[string]any)
	for k, v := range stats {
		combined[k] = v
	}
	for k, v := range debugStats {
		combined["debug."+k] = v
	}
	return combined
}
func (c *ControlAdapter) OnReload(fn func()) {
	c.config.OnReload(fn)
	control.RegisterReloadHook(fn)
}
func (c *ControlAdapter) SetMetric(key string, value any) {
	c.metrics.Set(key, value)
}
func (c *ControlAdapter) RegisterDebugProbe(name string, fn func() any) {
	c.debug.RegisterProbe(name, fn)
}
