// Package control
// Author: momentics <momentics@gmail.com>
//
// Hot-reload, runtime metrics, configuration control, and debug introspection layer.
// Part of hioload-ws high-load architecture core.
//
// Provides concurrent-safe state handling primitives including:
//   - Immutable snapshot config reads and atomic updates
//   - Runtime observers for hot-reload
//   - Metrics telemetry contracts
//   - State export, debug hooks, and probe registration
//
// This package is cross-platform and build-tag-partitioned as needed.
package control
