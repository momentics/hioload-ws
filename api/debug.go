// Package api
// Author: momentics <momentics@gmail.com>
//
// Live debug and contract validation support for production-grade workloads.

package api

// Debug exposes runtime introspection and health probes.
type Debug interface {
    // DumpState emits a snapshot of internal state and runtime metrics.
    // Intended for diagnostics and profiling, it should be fast and non-blocking.
    DumpState() map[string]any

    // RegisterProbe dynamically registers a named probe function.
    // The probe can be invoked during debug dumps, health checks, etc.
    RegisterProbe(name string, fn func() any)
}
