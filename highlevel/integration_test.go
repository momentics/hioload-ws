// Package highlevel provides tests for the high-level WebSocket library.
package highlevel

import (
	"testing"
)

// TestZeroCopySemantics tests that zero-copy concepts are preserved in high-level API
func TestZeroCopySemantics(t *testing.T) {
	// This test would verify that the high-level API maintains zero-copy behavior
	// by checking that buffers are properly managed and released

	// For now, we implement basic checks
	t.Run("BufferPoolIntegration", func(t *testing.T) {
		// Verify that the connection properly uses buffer pool
		// This would require access to internals in a full implementation
		t.Skip("Buffer pool integration test requires full implementation")
	})

	t.Run("AutoReleaseFunctionality", func(t *testing.T) {
		// Verify that buffers are automatically released
		t.Skip("Auto-release test requires full implementation")
	})
}

// TestNUMAAwarenessPreservation tests that NUMA awareness is preserved
func TestNUMAAwarenessPreservation(t *testing.T) {
	// This test would verify that NUMA awareness is preserved in the high-level API
	t.Run("ServerNUMAConfiguration", func(t *testing.T) {
		s := NewServer(":0")
		WithNUMANode(0)(s) // Apply NUMA option
		// Verify that NUMA configuration is preserved
		if s.cfg.NUMANode != 0 {
			t.Errorf("NUMA node not set properly, got %d", s.cfg.NUMANode)
		}
	})
	
	t.Run("ClientNUMAConfiguration", func(t *testing.T) {
		// This would test client NUMA configuration
		t.Skip("Client NUMA test requires full client implementation")
	})
}

// TestBatchProcessingPreservation tests that batch processing is preserved
func TestBatchProcessingPreservation(t *testing.T) {
	t.Run("ServerBatchConfiguration", func(t *testing.T) {
		s := NewServer(":0")
		WithBatchSize(32)(s) // Apply batch size option
		// Verify that batch size configuration is preserved
		if s.cfg.BatchSize != 32 {
			t.Errorf("Batch size not set properly, got %d", s.cfg.BatchSize)
		}
	})
}

// TestHighLoadConfiguration tests high-load related configurations
func TestHighLoadConfiguration(t *testing.T) {
	t.Run("ConnectionLimit", func(t *testing.T) {
		s := NewServer(":0")
		WithMaxConnections(1000)(s)
		if s.cfg.MaxConnections != 1000 {
			t.Errorf("Max connections not set properly, got %d", s.cfg.MaxConnections)
		}
	})
	
	t.Run("ChannelCapacity", func(t *testing.T) {
		s := NewServer(":0")
		WithChannelCapacity(1024)(s)
		if s.cfg.ChannelCapacity != 1024 {
			t.Errorf("Channel capacity not set properly, got %d", s.cfg.ChannelCapacity)
		}
	})
}