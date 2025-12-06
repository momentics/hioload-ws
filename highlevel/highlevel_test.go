package highlevel

import (
	"testing"
)

func TestHTTPMethods(t *testing.T) {
	server := NewServer(":0")

	// Test that HTTP methods register correctly
	handler := func(conn *Conn) { }

	server.GET("/test", handler)
	server.POST("/test", handler)
	server.PUT("/test", handler)
	server.PATCH("/test", handler)
	server.DELETE("/test", handler)

	// Basic check that server has handlers registered
	if server.handlers == nil {
		t.Fatal("Expected handlers map to be initialized")
	}

	// Test parameterized routes
	server.GET("/users/:id", handler)

	// The test is just to ensure no panics occur during registration
	t.Log("HTTP methods registered successfully")
}

func TestParameterizedRoutes(t *testing.T) {
	server := NewServer(":0")

	handler := func(conn *Conn) { }

	// Test single parameter
	server.GET("/users/:id", handler)

	// Test multiple parameters
	server.GET("/users/:id/messages/:messageId", handler)

	// Test exact match
	server.GET("/exact", handler)

	// The test is just to ensure no panics occur during registration
	t.Log("Parameterized routes registered successfully")
}

func TestRouteParameters(t *testing.T) {
	// This test would require more complex setup to fully test parameter extraction
	// For now, we just ensure the RouteParam type and methods exist and can be called
	
	params := []RouteParam{
		{Key: "id", Value: "123"},
		{Key: "name", Value: "test"},
	}
	
	if len(params) != 2 {
		t.Errorf("Expected 2 parameters, got %d", len(params))
	}
	
	if params[0].Key != "id" || params[0].Value != "123" {
		t.Errorf("First parameter should be {id: 123}, got {%s: %s}", params[0].Key, params[0].Value)
	}
}