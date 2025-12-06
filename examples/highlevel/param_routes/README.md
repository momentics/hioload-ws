# Parameterized Routes Example

This example demonstrates how to use parameterized routes in the hioload-ws high-level API, similar to Gin framework.

## Available Routes

- `/users/:id` - Handles WebSocket connections to user-specific endpoints
- `/users/:id/messages/:messageId` - Handles WebSocket connections with multiple parameters
- `/echo` - Traditional static route

## Usage

```bash
go run examples/highlevel/param_routes/main.go
```

The server will listen on `:8080` and handle connections to the parameterized routes.

## Key Features Demonstrated

1. **Parameter Extraction**: Using `conn.Param("paramName")` to get parameter values
2. **Multiple Parameters**: Routes with multiple parameters like `/users/:id/messages/:messageId`  
3. **Mixed Routing**: Combination of parameterized and static routes
4. **Dynamic Response**: Responses that include extracted parameter values