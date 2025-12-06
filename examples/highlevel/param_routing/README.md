# Parameterized Routing Example

This example demonstrates parameterized URL routing in the hioload-ws high-level API.

## Available Routes

- `GET /echo` - Static echo WebSocket endpoint
- `GET /users/:id` - User-specific endpoint with single parameter
- `GET /users/:id/messages/:messageId` - Message-specific endpoint with multiple parameters
- `GET /api/v1/users/:userId/orders/:orderId/items/:itemId` - Complex nested parameters
- `GET /api/v1/users/:id/profile` - JSON endpoint with parameter

## Usage

```bash
go run examples/highlevel/param_routing/main.go
```

The server will listen on `:8080` and demonstrate parameterized routing.

## Key Features Demonstrated

1. **Single Parameter Routes**: `/users/:id` - extracts `id` parameter
2. **Multiple Parameter Routes**: `/users/:id/messages/:messageId` - extracts both `id` and `messageId`
3. **Complex Nested Parameters**: Deep nesting with multiple parameters
4. **JSON Parameter Support**: Using parameters with JSON messaging
5. **Parameter Access**: Using `conn.Param("paramName")` to extract parameter values
6. **Integration**: Parameterized routes working with existing high-level features (message handling, etc.)

## Parameter Extraction

Parameters are extracted from the URL pattern and made available through the `conn.Param("name")` method:

```go
server.HandleFunc("/users/:id", func(conn *highlevel.Conn) {
    userId := conn.Param("id")
    // userId now contains the value from the URL path
})
```

The parameterized routing works with all message types (text, binary, JSON) and integrates seamlessly with other high-level API features.