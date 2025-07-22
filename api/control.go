// File: api/control.go
// Package api defines Control interface.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// Control manages dynamic config and runtime metrics.
type Control interface {
	GetConfig() map[string]any
	SetConfig(cfg map[string]any) error
	Stats() map[string]any
	OnReload(fn func())
	RegisterDebugProbe(name string, fn func() any)
}
