# Built-in Middleware Example

This example demonstrates the built-in middleware in the hioload-ws high-level API.

## Built-in Middleware

- **LoggingMiddleware**: Logs connection start/end information
- **RecoveryMiddleware**: Recovers from panics in handlers
- **MetricsMiddleware**: Collects connection metrics

## Available Routes

- `GET /echo` - Echo WebSocket endpoint with all built-in middleware
- `GET /users/:id` - User-specific endpoint with all built-in middleware

## Usage

```bash
go run examples/highlevel/built_in_middleware/main.go
```

The server will listen on `:8080` and apply built-in middleware to connections.

## Key Features Demonstrated

1. **Built-in Middleware**: Using `LoggingMiddleware`, `RecoveryMiddleware`, `MetricsMiddleware`
2. **Multiple Middleware**: Combining all three built-in middleware
3. **Metrics Collection**: Using `GetMetrics()` to retrieve server metrics
4. **Parameter Access**: Using middleware with parameterized routes
5. **Error Handling**: Recovery middleware handling panics gracefully
6. **Logging**: Automatic connection logging