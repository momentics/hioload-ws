// File: pool/bufferpool_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux NUMA-aware buffer pool using mmap with huge pages.
// All logic is safe to fallback if huge pages or NUMA are not present.

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

func (l *linuxBuffer) Bytes() []byte {
	return l.data
}
func (l *linuxBuffer) Copy() []byte {
	dst := make([]byte, len(l.data))
	copy(dst, l.data)
	return dst
}
func (l *linuxBuffer) NUMANode() int {
	return l.numaID
}
func (l *linuxBuffer) Release() {
	l.pool.recycle(l)
}
func (l *linuxBuffer) Slice(from, to int) api.Buffer {
	if from < 0 || to > len(l.data) || from > to {
		return nil
	}
	nb := *l
	nb.data = nb.data[from:to]
	return &nb
}

type linuxBufferPool struct {
	pools map[int]chan *linuxBuffer // NUMA node -> buffer channel
	mu    sync.Mutex
}

func newBufferPool(numaNode int) api.BufferPool {
	bp := &linuxBufferPool{pools: make(map[int]chan *linuxBuffer)}
	bp.pools[numaNode] = make(chan *linuxBuffer, 1024)
	return bp
}

// Get always returns a buffer sized >= size, possibly new or recycled.
func (bp *linuxBufferPool) Get(size, numaPreferred int) api.Buffer {
	bp.mu.Lock()
	ch, ok := bp.pools[numaPreferred]
	if !ok {
		ch = make(chan *linuxBuffer, 1024)
		bp.pools[numaPreferred] = ch
	}
	bp.mu.Unlock()
	select {
	case buf := <-ch:
		if cap(buf.data) < size {
			buf.data = make([]byte, size)
		} else {
			buf.data = buf.data[:size]
		}
		return buf
	default:
		const HUGEPAGE = 2 << 20
		length := ((size + HUGEPAGE - 1) / HUGEPAGE) * HUGEPAGE
		// try hugepage mmap; fallback allocation if needed
		data, err := syscall.Mmap(-1, 0, length, syscall.PROT_READ|syscall.PROT_WRITE,
			syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS|syscall.MAP_HUGETLB)
		if err != nil {
			data = make([]byte, size)
			return &linuxBuffer{data: data, pool: bp, numaID: numaPreferred}
		}
		return &linuxBuffer{data: data[:size], pool: bp, numaID: numaPreferred}
	}
}

// Put returns buffer to pool (non-blocking).
func (bp *linuxBufferPool) Put(b api.Buffer) {
	if lb, ok := b.(*linuxBuffer); ok {
		select {
		case bp.pools[lb.numaID] <- lb:
		default:
			// drop buffer if pool full
		}
	}
}

// Stats returns stats if required. Left empty for prototype.
func (bp *linuxBufferPool) Stats() api.BufferPoolStats {
	return api.BufferPoolStats{}
}

func (bp *linuxBufferPool) recycle(b *linuxBuffer) {
	bp.Put(b)
}
