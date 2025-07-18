// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package pool

import "sync"

// ObjectPool is a generic object pool.
type ObjectPool[T any] interface {
    Get() T
    Put(T)
}

// SyncPool wraps sync.Pool for generic usage.
type SyncPool[T any] struct {
    pool *sync.Pool
}

// NewSyncPool creates a new SyncPool with a creator function.
func NewSyncPool[T any](creator func() T) *SyncPool[T] {
    return &SyncPool[T]{
        pool: &sync.Pool{New: func() any { return creator() }},
    }
}

func (sp *SyncPool[T]) Get() T {
    return sp.pool.Get().(T)
}

func (sp *SyncPool[T]) Put(obj T) {
    sp.pool.Put(obj)
}
