// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// control_mock.go — Mock implementation of api.Control for tests.
package mocks

type ControlMock struct {
	Config map[string]any
	StatsMap map[string]any
}

func (c *ControlMock) GetConfig() map[string]any { return c.Config }
func (c *ControlMock) SetConfig(cfg map[string]any) error {
	for k, v := range cfg {
		c.Config[k] = v
	}
	return nil
}
func (c *ControlMock) Stats() map[string]any { return c.StatsMap }
func (c *ControlMock) OnReload(fn func())    {}
func (c *ControlMock) RegisterDebugProbe(string, func() any) {}
