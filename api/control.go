// File: api/control.go
// Package api
// Author: momentics
//
// Runtime configuration, statistics, dynamic reload and debug contract for highload systems.

package api

// Control exposes configuration, live metrics and debug API.
type Control interface {
    // GetConfig returns a snapshot of all configuration settings.
    GetConfig() map[string]any

    // SetConfig atomically updates or merges configuration settings.
    SetConfig(cfg map[string]any) error

    // Stats returns current aggregated runtime and performance metrics.
    Stats() map[string]any

    // OnReload registers a callback for hot-reload/config updates.
    OnReload(fn func())

    // RegisterDebugProbe dynamically registers a named debug probe function.
    // The probe is invoked during debug dumps and health checks.
    RegisterDebugProbe(name string, fn func() any)
}
