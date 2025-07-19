// Package api
// Author: momentics
//
// Runtime configuration, statistics, and dynamic reload contract for highload systems.

package api

// Control exposes configuration and live metrics API for runtime systems.
type Control interface {
    // GetConfig returns a snapshot of all configuration settings.
    GetConfig() map[string]any

    // SetConfig atomically updates or merges configuration settings.
    SetConfig(cfg map[string]any) error

    // Stats returns current aggregated runtime and performance metrics.
    Stats() map[string]any

    // OnReload registers a callback for hot-reload/config updates.
    OnReload(fn func())
}
