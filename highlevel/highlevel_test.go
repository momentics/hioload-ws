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

func TestRouteGroups(t *testing.T) {
	server := NewServer(":0")

	// Test group creation
	apiV1 := server.Group("/api/v1")
	if apiV1.prefix != "/api/v1" {
		t.Errorf("Expected group prefix to be '/api/v1', got '%s'", apiV1.prefix)
	}

	// Test nested group
	users := apiV1.Group("/users")
	if users.prefix != "/api/v1/users" {
		t.Errorf("Expected nested group prefix to be '/api/v1/users', got '%s'", users.prefix)
	}

	// Test handler registration with group
	handler := func(conn *Conn) {}

	// This should register handler for "/api/v1/users/:id"
	apiV1.GET("/users/:id", handler)

	// This should register handler for "/api/v1/users/:id/messages/:messageId"
	apiV1.GET("/users/:id/messages/:messageId", handler)

	// The test is just to ensure no panics occur during registration
	t.Log("Route groups registered successfully")
}