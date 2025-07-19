// Package api
// Author: momentics@gmail.com
//
// Transport interface for high performance, zero-copy, cross-platform message transfer.
// Should support: user-space networking, DPDK-offload, shared memory, lock-free IO.

package api

// Transport abstracts a sender/receiver capable of high-performance transmission.
type Transport interface {
    // Send pushes a set of buffers in zero-copy/batched fashion.
    Send(buffers [][]byte) error

    // Recv fetches the next batch/set of buffers.
    Recv() ([][]byte, error)

    // Close tears down transport and releases all resources.
    Close() error

    // Features returns the transport's capabilities (zero-copy, batching, NUMA-affinity, etc).
    Features() TransportFeatures
}

// TransportFeatures describes special capabilities of the underlying transport.
type TransportFeatures struct {
    ZeroCopy     bool
    Batch        bool
    NUMAAware    bool
    LockFree     bool
    SharedMemory bool
    OS           []string // Supported OSes
}
