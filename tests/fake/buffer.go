// Package fake provides mock implementations for testing hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package fake

import (
	"github.com/momentics/hioload-ws/api"
)

// FakeBuffer implements api.Buffer for testing.
type FakeBuffer struct {
	data     []byte
	numaNode int
	released bool
}

// NewFakeBuffer creates a new fake buffer with given data.
func NewFakeBuffer(data []byte, numaNode int) *FakeBuffer {
	if data == nil {
		data = make([]byte, 0)
	}
	return &FakeBuffer{
		data:     data,
		numaNode: numaNode,
		released: false,
	}
}

func (fb *FakeBuffer) Bytes() []byte {
	return fb.data
}

func (fb *FakeBuffer) Slice(from, to int) api.Buffer {
	// Bounds checking
	if from < 0 {
		from = 0
	}
	if to > len(fb.data) {
		to = len(fb.data)
	}
	if from > to {
		from, to = to, from
	}
	return &FakeBuffer{
		data:     fb.data[from:to],
		numaNode: fb.numaNode,
	}
}

func (fb *FakeBuffer) Release() {
	fb.released = true
}

func (fb *FakeBuffer) Copy() []byte {
	result := make([]byte, len(fb.data))
	copy(result, fb.data)
	return result
}

func (fb *FakeBuffer) NUMANode() int {
	return fb.numaNode
}

func (fb *FakeBuffer) Capacity() int {
	return cap(fb.data)
}

// IsReleased returns whether the buffer has been released.
func (fb *FakeBuffer) IsReleased() bool {
	return fb.released
}

// FakeBufferPool implements api.BufferPool for testing.
type FakeBufferPool struct {
	GetFunc func(size int, numaPreferred int) api.Buffer
	PutFunc func(b api.Buffer)
	StatsFunc func() api.BufferPoolStats
}

func (fp *FakeBufferPool) Get(size int, numaPreferred int) api.Buffer {
	if fp.GetFunc != nil {
		return fp.GetFunc(size, numaPreferred)
	}
	return NewFakeBuffer(make([]byte, size), numaPreferred)
}

func (fp *FakeBufferPool) Put(b api.Buffer) {
	if fp.PutFunc != nil {
		fp.PutFunc(b)
	}
}

func (fp *FakeBufferPool) Stats() api.BufferPoolStats {
	if fp.StatsFunc != nil {
		return fp.StatsFunc()
	}
	return api.BufferPoolStats{}
}