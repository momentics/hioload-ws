// File: pool/numapool.go
// Author: momentics <momentics@gmail.com>
//
// Platform-neutral NUMA-aware pool for memory allocation. Concrete allocators
// are selected at runtime through platform-specific factory in separate files.

package pool

import (
	"sync"
)

// NUMAAllocator defines interface for NUMA-aware memory allocators.
type NUMAAllocator interface {
	Alloc(size int, node int) ([]byte, error)
	Free([]byte)
	Nodes() (int, error)
}

// NUMAPool provides NUMA-aware allocation for []byte slices.
type NUMAPool struct {
	alloc  NUMAAllocator
	size   int
	pool   sync.Pool
	node   int // NUMA node
	enable bool
}

// NewNUMAPool creates a new NUMA-aware pool for target NUMA node.
// If NUMA is not available on this platform, fallback allocator is used.
func NewNUMAPool(node int, size int, enable bool) *NUMAPool {
	na := createNUMAAllocator()
	return &NUMAPool{
		alloc:  na,
		size:   size,
		node:   node,
		enable: enable && na != nil,
		pool: sync.Pool{
			New: func() interface{} {
				if na == nil || !enable {
					return make([]byte, size)
				}
				b, err := na.Alloc(size, node)
				if err != nil {
					return make([]byte, size)
				}
				return b
			},
		},
	}
}

// Get returns a buffer from the pool.
func (p *NUMAPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a buffer to the pool.
func (p *NUMAPool) Put(buf []byte) {
	if p.alloc != nil && p.enable {
		p.alloc.Free(buf)
	}
	p.pool.Put(buf[:p.size])
}
