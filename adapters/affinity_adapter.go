// Package adapters
// Author: momentics <momentics@gmail.com>
//
// Affinity adapter implementing api.Affinity interface using internal concurrency primitives.
// Provides CPU and NUMA pinning capabilities for high-performance computing environments.

package adapters

import (
	"runtime"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// AffinityAdapter implements api.Affinity using internal concurrency pinning functionality.
type AffinityAdapter struct {
	currentCPU  int
	currentNUMA int
	pinned      bool
	scope       api.AffinityScope
}

// NewAffinityAdapter creates a new affinity adapter with thread scope by default.
func NewAffinityAdapter() api.Affinity {
	return &AffinityAdapter{
		currentCPU:  -1,
		currentNUMA: -1,
		pinned:      false,
		scope:       api.ScopeThread,
	}
}

// Pin attaches the calling entity to a specific CPU and/or NUMA node.
// cpuID: -1 means "any CPU", numaID: -1 means "any NUMA node".
func (a *AffinityAdapter) Pin(cpuID int, numaID int) error {
	if cpuID == -1 {
		cpuID = concurrency.PreferredCPUID(numaID)
	}
	if numaID == -1 {
		numaID = concurrency.CurrentNUMANodeID()
	}

	concurrency.PinCurrentThread(numaID, cpuID)

	a.currentCPU = cpuID
	a.currentNUMA = numaID
	a.pinned = true

	return nil
}

// Unpin detaches the affinity; the execution may be migrated by the OS.
func (a *AffinityAdapter) Unpin() error {
	// On most systems, setting affinity to all CPUs effectively unpins
	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		concurrency.PinCurrentThread(-1, i)
	}

	a.pinned = false
	a.currentCPU = -1
	a.currentNUMA = -1

	return nil
}

// Get returns the effective (not just requested) CPU and NUMA node.
func (a *AffinityAdapter) Get() (cpuID int, numaID int, err error) {
	return a.currentCPU, a.currentNUMA, nil
}

// Scope returns the affinity's applicability (process/thread/goroutine).
func (a *AffinityAdapter) Scope() api.AffinityScope {
	return a.scope
}

// ImmutableDescriptor returns a snapshot for statistics/auditing.
func (a *AffinityAdapter) ImmutableDescriptor() api.AffinityDescriptor {
	return api.AffinityDescriptor{
		CPUID:  a.currentCPU,
		NUMAID: a.currentNUMA,
		Scope:  a.scope,
		Pinned: a.pinned,
	}
}
