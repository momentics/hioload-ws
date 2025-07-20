// File: api/affinity.go
// Package api defines CPU/NUMA affinity interface.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// AffinityScope enumerates binding scope.
type AffinityScope int

const (
	ScopeProcess AffinityScope = iota
	ScopeThread
	ScopeGoroutine
)

// AffinityDescriptor captures current binding state.
type AffinityDescriptor struct {
	CPUID  int
	NUMAID int
	Scope  AffinityScope
	Pinned bool
}

// Affinity defines CPU/NUMA binding contract.
type Affinity interface {
	// Pin assigns the current entity to specific CPU and/or NUMA node.
	Pin(cpuID, numaID int) error
	// Unpin clears any binding constraints.
	Unpin() error
	// Get reports effective CPU and NUMA.
	Get() (cpuID, numaID int, err error)
	// Scope returns binding scope.
	Scope() AffinityScope
	// ImmutableDescriptor returns descriptor snapshot.
	ImmutableDescriptor() AffinityDescriptor
}
