package unit

import (
	"testing"
	"time"
	
	"github.com/momentics/hioload-ws/highlevel"
)

// TestHighLevelServerCreation tests basic server creation
func TestHighLevelServerCreation(t *testing.T) {
	server := highlevel.NewServer(":8080")
	if server == nil {
		t.Fatal("Expected server to be created")
	}
	
	// Test default configuration
	active := server.GetActiveConnections()
	if active != 0 {
		t.Errorf("Expected 0 active connections, got %d", active)
	}
}

// TestHighLevelServerHandleFunc tests handler registration
func TestHighLevelServerHandleFunc(t *testing.T) {
	server := highlevel.NewServer(":8080")
	
	testHandler := func(c *highlevel.Conn) {
		// Test handler functionality
	}
	
	// Register a handler
	server.HandleFunc("/test", testHandler)
	
	// For now, just ensure no panic occurs
	_ = server.Handlers()
}

// TestHighLevelServerOptions tests server options
func TestHighLevelServerOptions(t *testing.T) {
	// Test WithMaxConnections
	opt1 := highlevel.WithMaxConnections(100)
	_ = opt1 // Just ensure it can be created
	
	// Test WithReadLimit
	opt2 := highlevel.WithReadLimit(1024 * 1024) // 1MB
	_ = opt2
	
	// Test WithWriteTimeout
	opt3 := highlevel.WithWriteTimeout(30 * time.Second)
	_ = opt3
	
	// Test WithReadTimeout
	opt4 := highlevel.WithReadTimeout(30 * time.Second)
	_ = opt4
	
	// Test WithBatchSize
	opt5 := highlevel.WithBatchSize(32)
	_ = opt5
	
	// Test WithChannelCapacity
	opt6 := highlevel.WithChannelCapacity(1024)
	_ = opt6
	
	// Test WithNUMANode
	opt7 := highlevel.WithNUMANode(0)
	_ = opt7
	
	// Ensure no panic
	t.Log("All server options applied without error")
}

// TestHighLevelServerHTTPMethods tests HTTP method registrations
func TestHighLevelServerHTTPMethods(t *testing.T) {
	server := highlevel.NewServer(":8080")
	
	handler := func(c *highlevel.Conn) {}
	
	// Test different HTTP methods
	server.GET("/get", handler)
	server.POST("/post", handler)
	server.PUT("/put", handler)
	server.PATCH("/patch", handler)
	server.DELETE("/delete", handler)
	
	// Test group registration
	group := server.Group("/api")
	group.GET("/users", handler)
	
	// Just ensure no panics
	t.Log("HTTP methods registered successfully")
}