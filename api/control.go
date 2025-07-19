// Package api
// Author: momentics@gmail.com
//
// Control plane for configuration, stats, and live reconfiguration.

package api

// Control exposes API for runtime configuration and metrics.
type Control interface {
    // GetConfig returns current config snapshot.
    GetConfig() map[string]any
    // SetConfig updates config.
    SetConfig(cfg map[string]any) error
    // Stats returns current performance metrics.
    Stats() map[string]any
    // OnReload registers reload hook.
    OnReload(fn func())
}
