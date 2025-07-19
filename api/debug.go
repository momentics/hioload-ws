// Package api
// Author: momentics
//
// Live debug and contract validation support for production workloads.

package api

// Debug exposes runtime introspection and health API.
type Debug interface {
    // DumpState emits a snapshot of system state for diagnostics.
    DumpState() map[string]any

    // RegisterProbe dynamically registers new debug probes.
    RegisterProbe(name string, fn func() any)
}
