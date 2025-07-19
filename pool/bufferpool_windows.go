// File: pool/bufferpool_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows large-page buffer pool using VirtualAlloc.
// Fallback allocs if large pages unavailable.

package pool

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"golang.org/x/sys/windows"
)

var (
	kern32           = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAlloc = kern32.NewProc("VirtualAlloc")
)

type windowsBuffer struct {
	data   []byte
	pool   *windowsBufferPool
	numaID int
}

func (w *windowsBuffer) Bytes() []byte {
	return w.data
}
func (w *windowsBuffer) Copy() []byte {
	dst := make([]byte, len(w.data))
	copy(dst, w.data)
	return dst
}
func (w *windowsBuffer) NUMANode() int {
	return w.numaID
}
func (w *windowsBuffer) Release() {
	w.pool.recycle(w)
}
func (w *windowsBuffer) Slice(from, to int) api.Buffer {
	if from < 0 || to > len(w.data) || from > to {
		return nil
	}
	nb := *w
	nb.data = nb.data[from:to]
	return &nb
}

type windowsBufferPool struct {
	pools map[int]chan *windowsBuffer
	mu    sync.Mutex
}

func newBufferPool(numaNode int) api.BufferPool {
	wp := &windowsBufferPool{pools: make(map[int]chan *windowsBuffer)}
	wp.pools[numaNode] = make(chan *windowsBuffer, 1024)
	return wp
}

func (bp *windowsBufferPool) Get(size, numaPreferred int) api.Buffer {
	bp.mu.Lock()
	ch, ok := bp.pools[numaPreferred]
	if !ok {
		ch = make(chan *windowsBuffer, 1024)
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
		// Try VirtualAlloc large page; fallback to slice
		addr, _, _ := procVirtualAlloc.Call(0, uintptr(size),
			windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES,
			syscall.PAGE_READWRITE)
		var slice []byte
		if addr == 0 {
			slice = make([]byte, size)
		} else {
			slice = (*[1 << 30]byte)(unsafe.Pointer(addr))[:size:size]
		}
		return &windowsBuffer{data: slice, pool: bp, numaID: numaPreferred}
	}
}

func (bp *windowsBufferPool) Put(b api.Buffer) {
	if wb, ok := b.(*windowsBuffer); ok {
		select {
		case bp.pools[wb.numaID] <- wb:
		default:
			// drop if pool full
		}
	}
}

func (bp *windowsBufferPool) Stats() api.BufferPoolStats {
	return api.BufferPoolStats{}
}

func (bp *windowsBufferPool) recycle(b *windowsBuffer) {
	bp.Put(b)
}
