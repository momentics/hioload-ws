// File: adapters/affinity_adapter.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
// Description:
//   Adapter exposing the api.Affinity interface, backed by internal concurrency primitives.

package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/internal/normalize"
)

// AffinityAdapter implements api.Affinity by delegating to internal concurrency.
type AffinityAdapter struct {
	currentCPU  int
	currentNUMA int
	pinned      bool
	scope       api.AffinityScope
}

// NewAffinityAdapter constructs a new AffinityAdapter with default thread scope.
func NewAffinityAdapter() api.Affinity {
	return &AffinityAdapter{
		currentCPU:  -1,
		currentNUMA: -1,
		pinned:      false,
		scope:       api.ScopeThread,
	}
}

// Pin binds the current OS thread to cpuID and/or numaID.
// If cpuID or numaID is -1, a reasonable default is chosen.
func (a *AffinityAdapter) Pin(cpuID, numaID int) error {
	node := normalize.NUMANodeAuto(numaID)
	cpu := normalize.CPUIndexAuto(cpuID)

	if err := concurrency.PinCurrentThread(node, cpu); err != nil {
		return err
	}
	a.currentCPU = cpuID
	a.currentNUMA = numaID
	a.pinned = true
	return nil
}

// Unpin releases any CPU/NUMA binding on this thread.
func (a *AffinityAdapter) Unpin() error {
	if err := concurrency.UnpinCurrentThread(); err != nil {
		return err
	}
	a.pinned = false
	a.currentCPU = -1
	a.currentNUMA = -1
	return nil
}

// Get returns the currently pinned CPU and NUMA node.
func (a *AffinityAdapter) Get() (cpuID, numaID int, err error) {
	return a.currentCPU, a.currentNUMA, nil
}

// Scope returns the binding scope (process, thread, or goroutine).
func (a *AffinityAdapter) Scope() api.AffinityScope {
	return a.scope
}

// ImmutableDescriptor returns a snapshot of the current binding state.
func (a *AffinityAdapter) ImmutableDescriptor() api.AffinityDescriptor {
	return api.AffinityDescriptor{
		CPUID:  a.currentCPU,
		NUMAID: a.currentNUMA,
		Scope:  a.scope,
		Pinned: a.pinned,
	}
}
