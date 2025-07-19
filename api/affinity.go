// Package api
// Author: momentics
//
// CPU/NUMA affinity, thread and process pinning abstractions for high-load, NUMA-aware networking.
//
// This interface enables hierarchical pinning (process, thread/goroutine, etc.)
// and fine-grained binding for efficiency in multicore and multi-NUMA node environments.

package api

// Affinity describes the runtime binding semantics for CPUs and NUMA nodes.
type Affinity interface {
    // Pin attaches the calling entity to a specific CPU and/or NUMA node,
    // maintaining execution locality for memory and network operations.
    // cpuID: -1 means "any CPU", numaID: -1 means "any NUMA node".
    Pin(cpuID int, numaID int) error

    // Unpin detaches the affinity; the execution may be migrated by the OS.
    Unpin() error

    // Get returns the effective (not just requested) CPU and NUMA node.
    Get() (cpuID int, numaID int, err error)

    // Scope returns the affinity's applicability (process/thread/goroutine).
    Scope() AffinityScope

    // ImmutableDescriptor returns a snapshot for statistics/auditing.
    ImmutableDescriptor() AffinityDescriptor
}

// AffinityScope enumerates levels of runtime binding.
type AffinityScope int

const (
    ScopeProcess AffinityScope = iota
    ScopeThread
    ScopeGoroutine
)

// AffinityDescriptor holds immutable info for monitoring.
type AffinityDescriptor struct {
    CPUID   int
    NUMAID  int
    Scope   AffinityScope
    Pinned  bool
}
