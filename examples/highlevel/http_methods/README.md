# HTTP Methods Example

This example demonstrates how to use HTTP method-based routing in the hioload-ws high-level API, similar to Gin framework.

## Available Routes

- `GET /ws/users/:id` - WebSocket connection for user-specific endpoints
- `POST /api/users` - Example of POST route (not usable for WebSocket, but shows API consistency)
- `PUT /api/users/:id` - Example of PUT route (not usable for WebSocket, but shows API consistency)  
- `GET /echo` - Traditional WebSocket echo endpoint
- `GET /chat` - Chat endpoint using default HandleFunc

## Usage

```bash
go run examples/highlevel/http_methods/main.go
```

The server will listen on `:8080` and handle connections based on HTTP methods.

## Important Note

WebSocket connections can only be established using the HTTP GET method (Upgrade request). Other HTTP methods (POST, PUT, DELETE, etc.) are not valid for establishing WebSocket connections, but are provided for API consistency and potential future extensions or hybrid HTTP/WebSocket servers.

## Key Features Demonstrated

1. **HTTP Method Routing**: Using `server.GET()`, `server.POST()`, `server.PUT()` etc.
2. **Parameter Extraction**: Using `conn.Param("paramName")` to get parameter values
3. **API Consistency**: Similar API design to popular frameworks like Gin
4. **Method Validation**: Internal method validation for route registration