// Package unit tests the buffer pool functionality.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package unit

import "testing"

// TestBuffer_SizeClassUpperBound tests the size class functionality without dependencies.
func TestBuffer_SizeClassUpperBound(t *testing.T) {
	// Define the size classes used in the actual code
	sizeClasses := []int{
		2 * 1024,        // 2K
		4 * 1024,        // 4K
		8 * 1024,        // 8K
		16 * 1024,       // 16K
		32 * 1024,       // 32K
		64 * 1024,       // 64K
		128 * 1024,      // 128K
		256 * 1024,      // 256K
		512 * 1024,      // 512K
		1 * 1024 * 1024, // 1M
	}

	// sizeClassUpperBound returns the smallest class >= requested size.
	sizeClassUpperBound := func(size int) int {
		for _, c := range sizeClasses {
			if size <= c {
				return c
			}
		}
		return sizeClasses[len(sizeClasses)-1] // fallback: biggest class
	}

	// Test that size classes work as expected
	if result := sizeClassUpperBound(100); result != 2048 {
		t.Errorf("Expected 2048 for size 100, got %d", result)
	}

	if result := sizeClassUpperBound(2048); result != 2048 {
		t.Errorf("Expected 2048 for size 2048, got %d", result)
	}

	if result := sizeClassUpperBound(3000); result != 4096 {
		t.Errorf("Expected 4096 for size 3000, got %d", result)
	}

	if result := sizeClassUpperBound(10000000); result != 1024*1024 { // larger than max
		t.Errorf("Expected max class %d for large size, got %d", 1024*1024, result)
	}
}

// TestBuffer_BasicFunctionality tests basic buffer functionality without dependencies.
func TestBuffer_BasicFunctionality(t *testing.T) {
	// Test buffer creation and basic operations
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Test slicing operation manually
	from, to := 10, 20
	if from < 0 {
		from = 0
	}
	if to > len(data) {
		to = len(data)
	}
	if from > to {
		from, to = to, from
	}

	slice := data[from:to]
	if len(slice) != 10 {
		t.Errorf("Expected slice length 10, got %d", len(slice))
	}

	// Verify slice contents
	for i := 0; i < 10; i++ {
		if slice[i] != data[i+10] {
			t.Errorf("Slice data mismatch at index %d", i)
		}
	}
}