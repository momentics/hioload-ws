// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows large-page buffer pool using VirtualAlloc.

package pool

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/momentics/hioload-ws/api"
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

// Bytes implements api.Buffer.
func (w *windowsBuffer) Bytes() []byte {
	panic("unimplemented")
}

// Copy implements api.Buffer.
func (w *windowsBuffer) Copy() []byte {
	panic("unimplemented")
}

// NUMANode implements api.Buffer.
func (w *windowsBuffer) NUMANode() int {
	panic("unimplemented")
}

// Release implements api.Buffer.
func (w *windowsBuffer) Release() {
	panic("unimplemented")
}

// Slice implements api.Buffer.
func (w *windowsBuffer) Slice(from int, to int) api.Buffer {
	panic("unimplemented")
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
	select {
	case buf := <-bp.pools[numaPreferred]:
		return buf
	default:
		addr, _, _ := procVirtualAlloc.Call(0, uintptr(size),
			windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES,
			syscall.PAGE_READWRITE)
		slice := (*[1 << 30]byte)(unsafe.Pointer(addr))[:size:size]
		return &windowsBuffer{data: slice, pool: bp, numaID: numaPreferred}
	}
}

func (bp *windowsBufferPool) Put(b api.Buffer) {
	if wb, ok := b.(*windowsBuffer); ok {
		bp.pools[wb.numaID] <- wb
	}
}

func (bp *windowsBufferPool) Stats() api.BufferPoolStats {
	return api.BufferPoolStats{}
}
