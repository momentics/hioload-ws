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

func TestMiddleware(t *testing.T) {
	server := NewServer(":0")

	// Test middleware registration
	executionOrder := []string{}

	// First middleware
	middleware1 := func(next func(*Conn)) func(*Conn) {
		return func(conn *Conn) {
			executionOrder = append(executionOrder, "middleware1_start")
			next(conn)
			executionOrder = append(executionOrder, "middleware1_end")
		}
	}

	// Second middleware
	middleware2 := func(next func(*Conn)) func(*Conn) {
		return func(conn *Conn) {
			executionOrder = append(executionOrder, "middleware2_start")
			next(conn)
			executionOrder = append(executionOrder, "middleware2_end")
		}
	}

	// Register middleware
	server.Use(middleware1, middleware2)

	// Create a test handler
	handler := func(conn *Conn) {
		executionOrder = append(executionOrder, "handler")
	}

	// Register a handler to test the chain
	server.GET("/test", handler)

	// Verify that middleware and handler functions are registered
	if len(server.middleware) != 2 {
		t.Errorf("Expected 2 middleware, got %d", len(server.middleware))
	}

	// The test is just to ensure no panics occur during registration and middleware application
	t.Log("Middleware registered successfully")
}

func TestGroupMiddleware(t *testing.T) {
	server := NewServer(":0")

	// Test that group Use method works
	apiV1 := server.Group("/api/v1")

	middleware := func(next func(*Conn)) func(*Conn) {
		return func(conn *Conn) {
			next(conn)
		}
	}

	// This should add middleware to the server (since group middleware affects the parent server)
	apiV1.Use(middleware)

	if len(server.middleware) != 1 {
		t.Errorf("Expected 1 middleware from group, got %d", len(server.middleware))
	}

	t.Log("Group middleware registered successfully")
}