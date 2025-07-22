//go:build linux
// +build linux

// File: pool/bufferpool_linux.go
// Package pool
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Платформенно-специфичная реализация linuxBufferPool.

package pool

import (
	"syscall"

	"github.com/momentics/hioload-ws/api"
)

type linuxBuffer struct {
	data   []byte
	pool   *baseBufferPool[*linuxBuffer]
	numaID int
}

func (b *linuxBuffer) Bytes() []byte { return b.data }
func (b *linuxBuffer) Slice(from, to int) api.Buffer {
	return &linuxBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID}
}
func (b *linuxBuffer) Release()      { b.pool.recycle(b) }
func (b *linuxBuffer) Copy() []byte  { c := make([]byte, len(b.data)); copy(c, b.data); return c }
func (b *linuxBuffer) NUMANode() int { return b.numaID }

func newLinuxBuffer(size, numaPref int) *linuxBuffer {
	length := ((size + (2 << 20) - 1) / (2 << 20)) * (2 << 20)
	data, err := syscall.Mmap(-1, 0, length,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE|syscall.MAP_HUGETLB)
	if err != nil {
		data = make([]byte, size)
	} else {
		data = data[:size]
	}
	return &linuxBuffer{data: data, numaID: numaPref}
}

func newBufferPool(numaNode int) api.BufferPool {
	var base *baseBufferPool[*linuxBuffer]
	factory := func(size, numaPref int) *linuxBuffer {
		buf := newLinuxBuffer(size, numaPref)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPool(numaNode, factory)
	return base
}
