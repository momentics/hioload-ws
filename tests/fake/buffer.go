package fake

import (
	"github.com/momentics/hioload-ws/api"
)

// FakeBuffer implements api.Buffer for testing.
type FakeBuffer struct {
	Data     []byte
	Node     int
	Released bool
}

// NewFakeBuffer creates a new fake buffer.
func NewFakeBuffer(size int, node int) *FakeBuffer {
	return &FakeBuffer{
		Data:     make([]byte, size),
		Node:     node,
		Released: false,
	}
}

func (fb *FakeBuffer) Bytes() []byte {
	return fb.Data
}

func (fb *FakeBuffer) Slice(from, to int) api.Buffer {
	if from < 0 || to > len(fb.Data) || from > to {
		return nil
	}
	return &FakeBuffer{
		Data:     fb.Data[from:to],
		Node:     fb.Node,
		Released: fb.Released,
	}
}

func (fb *FakeBuffer) Release() {
	fb.Released = true
}

func (fb *FakeBuffer) Copy() []byte {
	cp := make([]byte, len(fb.Data))
	copy(cp, fb.Data)
	return cp
}

func (fb *FakeBuffer) NUMANode() int {
	return fb.Node
}

func (fb *FakeBuffer) Capacity() int {
	return len(fb.Data)
}

func (fb *FakeBuffer) IsReleased() bool {
	return fb.Released
}

// FakeBufferPool implements api.BufferPool for testing.
type FakeBufferPool struct {
	Size   int
	Pools  []api.Buffer
	Buffer *FakeBuffer
}

// NewFakeBufferPool creates a new fake buffer pool.
func NewFakeBufferPool(size int) *FakeBufferPool {
	return &FakeBufferPool{
		Size: size,
		Buffer: &FakeBuffer{
			Data: make([]byte, size),
			Node: 0,
		},
	}
}

func (fbp *FakeBufferPool) Get(size int, numaPreferred int) api.Buffer {
	if size <= fbp.Size {
		return &FakeBuffer{
			Data: make([]byte, size),
			Node: numaPreferred,
		}
	}
	return &FakeBuffer{
		Data: make([]byte, size),
		Node: numaPreferred,
	}
}

func (fbp *FakeBufferPool) Put(b api.Buffer) {
	// For testing purposes, just store it
	fbp.Pools = append(fbp.Pools, b)
}

func (fbp *FakeBufferPool) Stats() api.BufferPoolStats {
	return api.BufferPoolStats{}
}