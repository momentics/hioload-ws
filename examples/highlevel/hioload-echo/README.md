# Hioload High-Level WebSocket Echo Server Example

This example demonstrates the high-level WebSocket API built on top of `hioload-ws` primitives. It provides the same high-performance benefits (zero-copy, NUMA-aware, batch processing) while offering a simple, easy-to-use interface similar to `gorilla/websocket`.

## Features

- **Simple API**: Minimal code for WebSocket applications
- **Zero-Copy**: Maintains performance through buffer pools
- **NUMA-Aware**: Configurable NUMA node selection
- **Batch Processing**: Optimized for high throughput
- **High Load**: Efficient resource management

## Comparison

### Original hioload-ws Approach (Complex)

```go
// Low-level approach (complex)
cfg := server.DefaultConfig()
cfg.ListenAddr = ":9001"
cfg.BatchSize = 32
srv, err := server.NewServer(cfg)
if err != nil { /* handle */ }

srv.Run(handler) // complex handler implementation
```

### New hioload-ws Approach (Simple)

```go
// High-level approach (simple)
server := hioload.NewServer(":8080")
server.HandleFunc("/echo", func(conn *hioload.Conn) {
    defer conn.Close()
    for {
        mt, message, err := conn.ReadMessage()
        if err != nil { break }
        conn.WriteMessage(mt, message)
    }
})
server.ListenAndServe()
```

## Running the Example

```bash
go run main.go -addr=":8080"
```

Then connect using any WebSocket client to `ws://localhost:8080/echo`.

## Key Benefits

1. **Less Code**: Fewer lines of code to implement the same functionality
2. **Automatic Resource Management**: Buffers are automatically released
3. **Safe by Default**: Protection against common mistakes
4. **Maintains Performance**: All low-level optimizations preserved
5. **Easy Migration**: Can gradually migrate from low-level API