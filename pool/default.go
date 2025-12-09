package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

var (
	defaultOnce sync.Once
	defaultMgr  *BufferPoolManager
)

// DefaultManager returns a process-wide BufferPoolManager so all components
// reuse the same NUMA-aware pools instead of fragmenting allocations.
func DefaultManager() *BufferPoolManager {
	defaultOnce.Do(func() {
		defaultMgr = NewBufferPoolManager(concurrency.NUMANodes())
	})
	return defaultMgr
}

// DefaultPool is a shortcut to fetch a pool from the default manager.
func DefaultPool(size, numaPreferred int) api.BufferPool {
	return DefaultManager().GetPool(size, numaPreferred)
}
