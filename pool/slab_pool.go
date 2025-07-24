// File: pool/slab_pool.go
// Package pool implements lock-free slab allocation with size class support.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// slabPool: fixed-size buffer allocation per size class/NUMA node.
type slabPool struct {
	size      int
	newBuf    func(size, numaNode int) api.Buffer
	release   func(api.Buffer)
	maxCached int64

	head       atomic.Pointer[nodeBuf]
	totalAlloc atomic.Uint64
	totalFree  atomic.Uint64
	numaStats  atomic.Pointer[numaMap]
}

// nodeBuf: node in lock-free stack.
type nodeBuf struct {
	buf  api.Buffer
	next *nodeBuf
}

// numaMap: allocation counters by NUMA node.
type numaMap struct {
	counts map[int]uint64
}

func newNumamap() *numaMap      { return &numaMap{counts: make(map[int]uint64)} }
func (m *numaMap) record(n int) { m.counts[n]++ }
func (m *numaMap) Get() map[int]uint64 {
	out := make(map[int]uint64, len(m.counts))
	for k, v := range m.counts {
		out[k] = v
	}
	return out
}

func (sp *slabPool) Get(_ int, numaNode int) api.Buffer {
	for {
		top := sp.head.Load()
		if top == nil {
			buf := sp.newBuf(sp.size, numaNode)
			if b, ok := buf.(*Buffer); ok {
				b.slab = sp
				b.class = sp.size // annotate buffer with size class
			}
			sp.totalAlloc.Add(1)
			mPtr := sp.numaStats.Load()
			if mPtr == nil {
				newMap := newNumamap()
				sp.numaStats.Store(newMap)
				mPtr = newMap
			}
			mPtr.record(numaNode)
			return buf
		}
		if sp.head.CompareAndSwap(top, top.next) {
			return top.buf
		}
	}
}

func (sp *slabPool) Put(buf api.Buffer) {
	freeCount := sp.totalFree.Load()
	if sp.maxCached > 0 && int64(freeCount) >= sp.maxCached {
		if sp.release != nil {
			sp.release(buf)
		}
		return
	}
	node := &nodeBuf{buf: buf}
	for {
		old := sp.head.Load()
		node.next = old
		if sp.head.CompareAndSwap(old, node) {
			sp.totalFree.Add(1)
			return
		}
	}
}

func (sp *slabPool) Stats() api.BufferPoolStats {
	totalAlloc := int64(sp.totalAlloc.Load())
	totalFree := int64(sp.totalFree.Load())
	inUse := totalAlloc - totalFree

	nm := sp.numaStats.Load()
	numaStats := make(map[int]int64)
	if nm != nil {
		raw := nm.Get()
		for node, cnt := range raw {
			numaStats[node] = int64(cnt)
		}
	}
	return api.BufferPoolStats{
		TotalAlloc: totalAlloc,
		TotalFree:  totalFree,
		InUse:      inUse,
		NUMAStats:  numaStats,
	}
}

var _ api.BufferPool = (*slabPool)(nil)
