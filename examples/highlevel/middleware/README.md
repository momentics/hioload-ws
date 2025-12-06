# Middleware Example

This example demonstrates how to use middleware in the hioload-ws high-level API, similar to Gin framework.

## Middleware Implemented

- **Logging Middleware**: Logs connection start/end times and duration
- **Recovery Middleware**: Recovers from panics in handlers
- **Authentication Middleware**: Example of auth middleware for protected routes

## Available Routes

- `GET /users/:id` - User-specific WebSocket connection with logging middleware
- `GET /api/v1/protected/:id` - Protected route with auth + logging + recovery middleware
- `GET /echo` - Echo WebSocket endpoint with logging middleware

## Usage

```bash
go run examples/highlevel/middleware/main.go
```

The server will listen on `:8080` and apply middleware to connections.

## Key Features Demonstrated

1. **Middleware Registration**: Using `server.Use(middleware...)` to add middleware to the chain
2. **Middleware Chaining**: Multiple middleware functions applied in order
3. **Group Middleware**: Adding middleware to route groups
4. **Custom Middleware**: Creating custom logging, recovery and authentication middleware
5. **Parameter Access in Middleware**: Using `conn.Param()` in middleware functions
6. **Connection Lifecycle**: Middleware wrapping the full connection lifecycle