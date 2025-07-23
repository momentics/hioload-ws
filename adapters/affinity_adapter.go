// File: adapters/affinity_adapter.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
// Description:
//   Adapter implementing the api.Affinity interface, delegating to
//   internal concurrency primitives for CPU and NUMA pinning.
//
// Package adapters provides glue code between the core API contracts
// and the internal implementation.

package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// AffinityAdapter implements api.Affinity using internal concurrency functions.
// It tracks current CPU and NUMA bindings and manages pin/unpin operations.
type AffinityAdapter struct {
	currentCPU  int
	currentNUMA int
	pinned      bool
	scope       api.AffinityScope
}

// NewAffinityAdapter creates a new AffinityAdapter with default thread scope.
// Default CPU and NUMA IDs are set to -1 (no binding).
func NewAffinityAdapter() api.Affinity {
	return &AffinityAdapter{
		currentCPU:  -1,
		currentNUMA: -1,
		pinned:      false,
		scope:       api.ScopeThread,
	}
}

// Pin assigns the calling entity (thread) to a specific CPU and/or NUMA node.
// cpuID: -1 means any CPU; numaID: -1 means any NUMA node.
func (a *AffinityAdapter) Pin(cpuID int, numaID int) error {
	// If cpuID is unspecified, choose preferred CPU for the given NUMA node
	if cpuID == -1 {
		cpuID = concurrency.PreferredCPUID(numaID)
	}
	// If numaID is unspecified, detect current NUMA node
	if numaID == -1 {
		numaID = concurrency.CurrentNUMANodeID()
	}

	// Delegate to internal implementation
	if err := concurrency.PinCurrentThread(numaID, cpuID); err != nil {
		return err
	}

	// Update state
	a.currentCPU = cpuID
	a.currentNUMA = numaID
	a.pinned = true
	return nil
}

// Unpin clears any CPU/NUMA binding, allowing the OS scheduler to migrate the thread.
func (a *AffinityAdapter) Unpin() error {
	// On Linux/Windows, use internal unpin to reset affinity
	if err := concurrency.UnpinCurrentThread(); err != nil {
		return err
	}
	a.pinned = false
	a.currentCPU = -1
	a.currentNUMA = -1
	return nil
}

// Get returns the currently effective CPU and NUMA IDs for this adapter.
func (a *AffinityAdapter) Get() (cpuID int, numaID int, err error) {
	return a.currentCPU, a.currentNUMA, nil
}

// Scope returns the binding scope (process, thread, or goroutine).
func (a *AffinityAdapter) Scope() api.AffinityScope {
	return a.scope
}

// ImmutableDescriptor returns a snapshot of the current binding state,
// useful for metrics, logging, or diagnostics.
func (a *AffinityAdapter) ImmutableDescriptor() api.AffinityDescriptor {
	return api.AffinityDescriptor{
		CPUID:  a.currentCPU,
		NUMAID: a.currentNUMA,
		Scope:  a.scope,
		Pinned: a.pinned,
	}
}
