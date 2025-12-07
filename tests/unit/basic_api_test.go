package unit

import (
	"testing"
	
	"github.com/momentics/hioload-ws/highlevel"
	"github.com/momentics/hioload-ws/lowlevel/server"
	"github.com/momentics/hioload-ws/lowlevel/client"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// TestAPIAvailability tests that core APIs are available
func TestAPIAvailability(t *testing.T) {
	// Test highlevel API availability
	highLevelServer := highlevel.NewServer(":8080")
	if highLevelServer == nil {
		t.Error("highlevel.NewServer should be available")
	}
	
	// Test that we can register a handler
	handler := func(conn *highlevel.Conn) {}
	highLevelServer.HandleFunc("/test", handler)
	
	t.Log("highlevel API is available")
}

// TestLowLevelServerAPI tests lowlevel server API availability
func TestLowLevelServerAPI(t *testing.T) {
	// Test default config creation
	cfg := server.DefaultConfig()
	if cfg == nil {
		t.Error("server.DefaultConfig should be available")
	}
	
	t.Log("server API is available")
}

// TestLowLevelClientAPI tests lowlevel client API availability
func TestLowLevelClientAPI(t *testing.T) {
	// Test default config creation
	cfg := client.DefaultConfig()
	if cfg == nil {
		t.Error("client.DefaultConfig should be available")
	}
	
	t.Log("client API is available")
}

// TestPoolAPI tests buffer pool API availability
func TestPoolAPI(t *testing.T) {
	// Test buffer pool manager creation
	manager := pool.NewBufferPoolManager(1)
	if manager == nil {
		t.Error("pool.NewBufferPoolManager should be available")
	}
	
	// Test getting a pool
	poolInstance := manager.GetPool(1024, 0)
	if poolInstance == nil {
		t.Error("pool.GetPool should be available")
	}
	
	// Test getting a buffer
	buffer := poolInstance.Get(512, 0)
	if buffer == nil {
		t.Error("pool.Get should be available")
	}
	
	t.Log("pool API is available")
}

// TestProtocolAPI tests protocol API availability
func TestProtocolAPI(t *testing.T) {
	// Test constants
	if protocol.MaxFramePayload <= 0 {
		t.Error("protocol.MaxFramePayload should be set")
	}
	
	if protocol.WebSocketGUID == "" {
		t.Error("protocol.WebSocketGUID should be set")
	}
	
	t.Log("protocol API is available")
}

// TestHighLevelServerCreation tests highlevel server creation
func TestHighLevelServerCreation(t *testing.T) {
	server := highlevel.NewServer(":9000")
	if server == nil {
		t.Error("highlevel.NewServer should create server")
	} else {
		t.Log("highlevel server created successfully")
	}
}

// TestHighLevelServerRegistration tests handler registration
func TestHighLevelServerRegistration(t *testing.T) {
	server := highlevel.NewServer(":9001")
	
	// Define test handler
	testHandler := func(c *highlevel.Conn) {
		// Do nothing for test
	}
	
	// Test various registration methods
	server.HandleFunc("/test", testHandler)
	server.GET("/get", testHandler)
	server.POST("/post", testHandler)
	
	// Test group creation
	group := server.Group("/api")
	group.GET("/users", testHandler)
	
	t.Log("highlevel server handler registration works")
}

// TestMessageTypes tests message type constants
func TestMessageTypes(t *testing.T) {
	// Test highlevel constants
	if highlevel.TextMessage != 1 {
		t.Errorf("Expected TextMessage to be 1, got %d", highlevel.TextMessage)
	}
	
	if highlevel.BinaryMessage != 2 {
		t.Errorf("Expected BinaryMessage to be 2, got %d", highlevel.BinaryMessage)
	}
	
	if highlevel.CloseMessage != 8 {
		t.Errorf("Expected CloseMessage to be 8, got %d", highlevel.CloseMessage)
	}
	
	if highlevel.PingMessage != 9 {
		t.Errorf("Expected PingMessage to be 9, got %d", highlevel.PingMessage)
	}
	
	if highlevel.PongMessage != 10 {
		t.Errorf("Expected PongMessage to be 10, got %d", highlevel.PongMessage)
	}
	
	t.Log("Message type constants are correct")
}

// TestHTTPMethods tests HTTP method constants
func TestHTTPMethods(t *testing.T) {
	methods := []highlevel.HTTPMethod{
		highlevel.GET,
		highlevel.POST,
		highlevel.PUT,
		highlevel.PATCH,
		highlevel.DELETE,
		highlevel.HEAD,
		highlevel.OPTIONS,
		highlevel.TRACE,
	}
	
	// Just verify they exist and are not empty when converted to string
	for i, method := range methods {
		if string(method) == "" {
			t.Errorf("HTTP method %d is empty", i)
		}
	}
	
	t.Log("HTTP method constants are available")
}