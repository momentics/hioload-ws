// File: pool/bufferpool_windows.go
// +build windows
// Package pool
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
    kern32           = windows.NewLazySystemDLL("kernel32.dll")
    procVirtualAlloc = kern32.NewProc("VirtualAlloc")
)

type windowsBuffer struct {
    data   []byte
    pool   *windowsBufferPool
    numaID int
}

func (b *windowsBuffer) Bytes() []byte               { return b.data }
func (b *windowsBuffer) Release()                    { b.pool.recycle(b) }
func (b *windowsBuffer) Copy() []byte                { c := make([]byte, len(b.data)); copy(c, b.data); return c }
func (b *windowsBuffer) NUMANode() int               { return b.numaID }
func (b *windowsBuffer) Slice(from, to int) api.Buffer { return &windowsBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID} }

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
        addr, _, err := procVirtualAlloc.Call(
            0, uintptr(size),
            windows.MEM_RESERVE|windows.MEM_COMMIT|0x20000000,
            windows.PAGE_READWRITE,
        )
        if addr == 0 || err != nil {
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
