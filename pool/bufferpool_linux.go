// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux NUMA-aware buffer pool using mmap with huge pages.

package pool

import (
	"sync"
	"syscall"

	"github.com/momentics/hioload-ws/api"
)

type linuxBuffer struct {
	data   []byte
	pool   *linuxBufferPool
	numaID int
}

// Bytes implements api.Buffer.
func (l *linuxBuffer) Bytes() []byte {
	panic("unimplemented")
}

// Copy implements api.Buffer.
func (l *linuxBuffer) Copy() []byte {
	panic("unimplemented")
}

// NUMANode implements api.Buffer.
func (l *linuxBuffer) NUMANode() int {
	panic("unimplemented")
}

// Release implements api.Buffer.
func (l *linuxBuffer) Release() {
	panic("unimplemented")
}

// Slice implements api.Buffer.
func (l *linuxBuffer) Slice(from int, to int) api.Buffer {
	panic("unimplemented")
}

type linuxBufferPool struct {
	pools map[int]chan *linuxBuffer
	mu    sync.Mutex
}

func newBufferPool(numaNode int) api.BufferPool {
	bp := &linuxBufferPool{pools: make(map[int]chan *linuxBuffer)}
	bp.pools[numaNode] = make(chan *linuxBuffer, 1024)
	return bp
}

func (bp *linuxBufferPool) Get(size, numaPreferred int) api.Buffer {
	bp.mu.Lock()
	ch := bp.pools[numaPreferred]
	bp.mu.Unlock()
	select {
	case buf := <-ch:
		if cap(buf.data) < size {
			buf.data = buf.data[:size]
		}
		return buf
	default:
		const HUGEPAGE = 2 << 20
		length := ((size + HUGEPAGE - 1) / HUGEPAGE) * HUGEPAGE
		data, _ := syscall.Mmap(-1, 0, length, syscall.PROT_READ|syscall.PROT_WRITE,
			syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS|syscall.MAP_HUGETLB)
		return &linuxBuffer{data: data[:size], pool: bp, numaID: numaPreferred}
	}
}

func (bp *linuxBufferPool) Put(b api.Buffer) {
	if lb, ok := b.(*linuxBuffer); ok {
		bp.pools[lb.numaID] <- lb
	}
}

func (bp *linuxBufferPool) Stats() api.BufferPoolStats {
	return api.BufferPoolStats{}
}
