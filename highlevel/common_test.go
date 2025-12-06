// Package highlevel provides tests for the high-level WebSocket library.
package highlevel

import (
	"testing"
)

// Test that the basic types and functions are available
func TestBasicAPI(t *testing.T) {
	// Test that we can create a server
	s := NewServer(":8080")
	if s == nil {
		t.Fatal("Failed to create server")
	}
	
	// Test constants
	if Version == "" {
		t.Error("Version should not be empty")
	}
	
	// Test error types
	if ErrClosed == nil {
		t.Error("ErrClosed should not be nil")
	}
	
	if ErrReadLimit == nil {
		t.Error("ErrReadLimit should not be nil")
	}
}

// Test that message types are defined
func TestMessageTypes(t *testing.T) {
	types := []MessageType{
		TextMessage,
		BinaryMessage,
		CloseMessage,
		PingMessage,
		PongMessage,
	}
	
	for i, mt := range types {
		if int(mt) != i+1 && !(i == 2 && int(mt) == 8) && !(i == 3 && int(mt) == 9) && !(i == 4 && int(mt) == 10) {
			// Special values for control messages
			t.Errorf("MessageType %d has unexpected value %d", i, mt)
		}
	}
}

// Test server options
func TestServerOptions(t *testing.T) {
	s := NewServer(":8080")

	// Apply some options
	opts := []ServerOption{
		WithMaxConnections(1000),
		WithBatchSize(32),
	}

	for _, opt := range opts {
		opt(s) // Apply option to server
	}

	// Basic check that options were applied
	if s.cfg.MaxConnections != 1000 {
		t.Errorf("MaxConnections not set properly, got %d", s.cfg.MaxConnections)
	}
}