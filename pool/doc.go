// Package pool
// Author: momentics <momentics@gmail.com>
//
// High-performance IO and Memory Layer for hioload-ws.
// Implements NUMA-aware, lock-free, zero-copy buffer pooling, batching, and ring buffering.
// All primitives are cross-platform (Linux/Windows) and designed for ultra-low-latency, high-throughput workloads.
// See bufferpool.go, batch.go, ring.go for implementation details.
package pool
