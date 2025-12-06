# Route Groups Example

This example demonstrates how to use route groups in the hioload-ws high-level API, similar to Gin framework.

## Available Routes

### API Version 1 (`/api/v1`)
- `GET /api/v1/users/:id` - WebSocket connection for user-specific endpoints (v1)
- `GET /api/v1/users/:id/messages/:messageId` - WebSocket connection with multiple parameters (v1)
- `GET /api/v1/chat/room/:roomId` - Chat room WebSocket connection (v1)

### API Version 2 (`/api/v2`)
- `GET /api/v2/users/:id` - WebSocket connection for user-specific endpoints (v2)

### Static Routes
- `GET /echo` - Traditional WebSocket echo endpoint

## Usage

```bash
go run examples/highlevel/route_groups/main.go
```

The server will listen on `:8080` and handle connections with route groups.

## Key Features Demonstrated

1. **Route Groups**: Using `server.Group("/api/v1")` to create grouped routes
2. **Nested Groups**: Using `apiV1.Group("/chat")` to create nested route groups
3. **Parameter Extraction**: Using `conn.Param("paramName")` within grouped routes
4. **API Versioning**: Different API versions with same route patterns using groups
5. **Method Consistency**: Using HTTP methods (GET) with grouped routes
6. **Mixed Routing**: Combination of grouped and non-grouped routes