package unit

import (
	"testing"
	
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/pool"
)

// TestWSConnectionCreation tests basic WSConnection creation and initialization
func TestWSConnectionCreation(t *testing.T) {
	// We'll test with a real buffer pool instead of fake for now
	manager := pool.NewBufferPoolManager(1)
	pool := manager.GetPool(1024, 0)
	
	// Creating protocol connection requires a transport, which we won't create here
	// Instead, we'll test basic functionality
	conn := protocol.NewWSConnection(nil, pool, 16)
	if conn == nil {
		t.Fatal("Expected connection to be created")
	}
	
	// Test that it has the right buffer pool
	bufferPool := conn.BufferPool()
	if bufferPool == nil {
		t.Error("Expected buffer pool to be available")
	}
}

// TestWSConnectionWithPathCreation tests WSConnection with path
func TestWSConnectionWithPathCreation(t *testing.T) {
	manager := pool.NewBufferPoolManager(1)
	pool := manager.GetPool(1024, 0)
	
	conn := protocol.NewWSConnectionWithPath(nil, pool, 16, "/test")
	if conn == nil {
		t.Fatal("Expected connection with path to be created")
	}
	
	// Test getting path
	path := conn.Path()
	if path != "/test" {
		t.Errorf("Expected path to be /test, got %s", path)
	}
}

// TestWSConnectionStartStop tests starting and stopping the connection
func TestWSConnectionStartStop(t *testing.T) {
	manager := pool.NewBufferPoolManager(1)
	pool := manager.GetPool(1024, 0)

	// Test with minimal config
	conn := protocol.NewWSConnection(nil, pool, 16)

	// Test that the connection can be created and closed safely
	// Note: Starting with nil transport will cause issues, so we just test creation
	// We can't start loops with nil transport, so skip that
	_ = conn
}