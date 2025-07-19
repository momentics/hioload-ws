// File: pool/doc.go
// Package pool
// Author: momentics <momentics@gmail.com>
//
// High-performance, cross-platform buffer pooling, batching, and ring buffer layer for hioload-ws.
// Implements NUMA-aware, zero-copy pools and batching primitives for all supported OS (Linux/Windows).
// DPDK compatibility and extension via interface layer.
// All core methods are thread-safe or explicitly document the concurrency contract.
package pool
