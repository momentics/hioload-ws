//go:build !linux && !windows
// +build !linux,!windows

// File: pool/numa_stub.go
// Author: momentics <momentics@gmail.com>
//
// Stub NUMA allocator for unsupported platforms.

package pool

// stubNUMAAllocator does nothing and always returns nil/error.
type stubNUMAAllocator struct{}

func (s *stubNUMAAllocator) Alloc(size int, node int) ([]byte, error) {
	return nil, nil
}

func (s *stubNUMAAllocator) Free([]byte) {}

func (s *stubNUMAAllocator) Nodes() (int, error) {
	return 1, nil
}

func newStubNUMAAllocator() NUMAAllocator {
	return &stubNUMAAllocator{}
}
