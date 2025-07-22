//go:build windows
// +build windows

// File: pool/bufferpool_windows.go
// Package pool
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Платформенно-специфичная реализация windowsBufferPool.

package pool

import (
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"golang.org/x/sys/windows"
)

var (
	procVirtualAllocExNuma = windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualAllocExNuma")
)

type windowsBuffer struct {
	data   []byte
	pool   *baseBufferPool[*windowsBuffer]
	numaID int
}

func (b *windowsBuffer) Bytes() []byte { return b.data }
func (b *windowsBuffer) Slice(from, to int) api.Buffer {
	return &windowsBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID}
}
func (b *windowsBuffer) Release()      { b.pool.recycle(b) }
func (b *windowsBuffer) Copy() []byte  { c := make([]byte, len(b.data)); copy(c, b.data); return c }
func (b *windowsBuffer) NUMANode() int { return b.numaID }

func newWindowsBuffer(size, numaPref int) *windowsBuffer {
	node := uint32(numaPref)
	addr, _, err := procVirtualAllocExNuma.Call(
		uintptr(windows.CurrentProcess()), 0,
		uintptr(size),
		uintptr(windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES),
		uintptr(windows.PAGE_READWRITE),
		uintptr(node),
	)
	if addr == 0 || err != nil {
		return &windowsBuffer{data: make([]byte, size), numaID: numaPref}
	}
	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	return &windowsBuffer{data: data, numaID: numaPref}
}

func newBufferPool(numaNode int) api.BufferPool {
	var base *baseBufferPool[*windowsBuffer]
	factory := func(size, numaPref int) *windowsBuffer {
		buf := newWindowsBuffer(size, numaPref)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPool(numaNode, factory)
	return base
}
