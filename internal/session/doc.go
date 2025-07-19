// Package session
// Author: momentics <momentics@gmail.com>
//
// High-performance session and context management layer.
// Each Session maps to a connected client or transport-level connection.
// Layer is NUMA-aware, zero-copy-safe, lock-free where appropriate.
//
// Context state is designed for propagation and high concurrency.
// Cancellation and TTL logic is included for graceful shutdown/lifecycle.

package session
