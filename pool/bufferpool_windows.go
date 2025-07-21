// File: pool/bufferpool_windows.go
//go:build windows
// +build windows

//
// Cross-platform NUMA-aware buffer pool for Windows.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"sync"
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"golang.org/x/sys/windows"
)

var (
	kern32                 = windows.NewLazySystemDLL("kernel32.dll")
	procVirtualAllocExNuma = kern32.NewProc("VirtualAllocExNuma")
)

type windowsBuffer struct {
	data   []byte
	pool   *windowsBufferPool
	numaID int
}

func (b *windowsBuffer) Bytes() []byte { return b.data }
func (b *windowsBuffer) Release()      { b.pool.recycle(b) }
func (b *windowsBuffer) Copy() []byte  { c := make([]byte, len(b.data)); copy(c, b.data); return c }
func (b *windowsBuffer) NUMANode() int { return b.numaID }
func (b *windowsBuffer) Slice(from, to int) api.Buffer {
	return &windowsBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID}
}

type windowsBufferPool struct {
	pools map[int]chan *windowsBuffer
	mu    sync.Mutex
}

func newBufferPool(numaNode int) api.BufferPool {
	return &windowsBufferPool{pools: map[int]chan *windowsBuffer{numaNode: make(chan *windowsBuffer, 1024)}}
}

func (p *windowsBufferPool) Get(size, numaPref int) api.Buffer {
	p.mu.Lock()
	ch := p.pools[numaPref]
	p.mu.Unlock()
	select {
	case buf := <-ch:
		if cap(buf.data) < size {
			buf.data = make([]byte, size)
		} else {
			buf.data = buf.data[:size]
		}
		return buf
	default:
		node := uint32(numaPref)
		addr, _, _ := procVirtualAllocExNuma.Call(
			uintptr(windows.CurrentProcess()),
			0,
			uintptr(size),
			uintptr(windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES),
			uintptr(windows.PAGE_READWRITE),
			uintptr(node),
		)
		if addr == 0 {
			// fallback
			return &windowsBuffer{data: make([]byte, size), pool: p, numaID: numaPref}
		}
		data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
		return &windowsBuffer{data: data, pool: p, numaID: numaPref}
	}
}

func (p *windowsBufferPool) Put(b api.Buffer) {
	if wb, ok := b.(*windowsBuffer); ok {
		select {
		case p.pools[wb.numaID] <- wb:
		default:
		}
	}
}

func (p *windowsBufferPool) Stats() api.BufferPoolStats { return api.BufferPoolStats{} }
func (p *windowsBufferPool) recycle(b *windowsBuffer)   { p.Put(b) }
