// control/debug.go
// Author: momentics <momentics@gmail.com>
//
// Runtime debug handler and probe reflector for internal inspection.

package control

import "sync"

// DebugProbes holds registered probe functions.
type DebugProbes struct {
	mu     sync.RWMutex
	probes map[string]func() any
}

// NewDebugProbes creates a probe registry.
func NewDebugProbes() *DebugProbes {
	return &DebugProbes{
		probes: make(map[string]func() any),
	}
}

// RegisterProbe inserts a named debug hook.
func (dp *DebugProbes) RegisterProbe(name string, fn func() any) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.probes[name] = fn
}

// DumpState returns output of all probes.
func (dp *DebugProbes) DumpState() map[string]any {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	out := make(map[string]any)
	for k, fn := range dp.probes {
		out[k] = fn()
	}
	return out
}
