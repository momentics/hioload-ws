package unit

import (
	"testing"
	
	"github.com/momentics/hioload-ws/pool"
)

// TestBufferPoolManager tests the buffer pool manager
func TestBufferPoolManager(t *testing.T) {
	numaCount := 1
	manager := pool.NewBufferPoolManager(numaCount)
	
	if manager == nil {
		t.Fatal("Expected buffer pool manager to be created")
	}
	
	// Test getting a pool
	pool1 := manager.GetPool(1024, 0)
	if pool1 == nil {
		t.Error("Expected to get a buffer pool")
	}
	
	// Test getting another pool (should return same if node count is 1)
	pool2 := manager.GetPool(2048, 0)
	if pool2 == nil {
		t.Error("Expected to get a buffer pool")
	}
	
	// Different node
	pool3 := manager.GetPool(512, 1)
	// This might be same as pool1 if node 1 maps to same pool as node 0
	if pool3 == nil {
		t.Error("Expected to get a buffer pool for node 1")
	}
}

// TestBufferPool tests individual buffer pool functionality
func TestBufferPool(t *testing.T) {
	numaCount := 1
	manager := pool.NewBufferPoolManager(numaCount)
	
	// Get a pool for a specific size class
	pool := manager.GetPool(1024, 0)
	if pool == nil {
		t.Fatal("Expected to get a buffer pool")
	}
	
	// Test getting a buffer
	buf := pool.Get(512, 0)
	if buf == nil {
		t.Error("Expected to get a buffer")
	}
	
	if len(buf.Bytes()) < 512 {
		t.Errorf("Expected buffer to have at least 512 bytes, got %d", len(buf.Bytes()))
	}
	
	// Test releasing the buffer back to pool
	buf.Release()
	
	// Get another buffer - it might come from the same pool
	buf2 := pool.Get(256, 0)
	if buf2 == nil {
		t.Error("Expected to get another buffer")
	}
	
	buf2.Release()
}

// TestMultiNodeBufferPool tests buffer pools with multiple NUMA nodes
func TestMultiNodeBufferPool(t *testing.T) {
	numaCount := 2
	manager := pool.NewBufferPoolManager(numaCount)
	
	if manager == nil {
		t.Fatal("Expected buffer pool manager to be created")
	}
	
	// Get pools for different NUMA nodes
	pool0 := manager.GetPool(1024, 0)
	pool1 := manager.GetPool(1024, 1)
	
	if pool0 == nil || pool1 == nil {
		t.Fatal("Expected to get pools for both NUMA nodes")
	}
	
	// Test getting buffers from different pools
	buf0 := pool0.Get(500, 0)
	buf1 := pool1.Get(500, 1)
	
	if buf0 == nil || buf1 == nil {
		t.Fatal("Expected to get buffers from both pools")
	}
	
	// Release buffers back to their respective pools
	buf0.Release()
	buf1.Release()
}