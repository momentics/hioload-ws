# Custom Middleware Example

This example demonstrates how to create custom middleware in the hioload-ws high-level API.

## Custom Middleware Implemented

- **CustomAuthMiddleware**: Example of authentication middleware
- **CustomRateLimitMiddleware**: Example of rate limiting middleware
- **CustomRequestIDMiddleware**: Example of request ID/logging middleware

## Available Routes

- `GET /echo` - Echo WebSocket endpoint with all custom and built-in middleware

## Usage

```bash
go run examples/highlevel/custom_middleware/main.go
```

The server will listen on `:8080` and demonstrate custom middleware usage.

## API for Custom Middleware

To create your own middleware, implement a function with the signature:

```go
func MyMiddleware(next func(*highlevel.Conn)) func(*highlevel.Conn) {
    return func(conn *highlevel.Conn) {
        // Pre-processing: Do something before the handler
        // Example: log, authenticate, validate, etc.
        
        // Call the next handler in the chain
        next(conn)
        
        // Post-processing: Do something after the handler
        // Example: log completion, cleanup, etc.
    }
}
```

## Key Features Demonstrated

1. **Custom Middleware Creation**: Implementing middleware with proper signature
2. **Middleware Composition**: Combining custom and built-in middleware
3. **Pre/Post Processing**: Demonstrating processing before and after handlers
4. **Error Handling**: Middleware working with error handling
5. **Chaining**: Multiple middleware functions applied in sequence
6. **Integration**: Custom middleware working with existing features (params, groups, etc.)