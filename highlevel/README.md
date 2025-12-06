# HighLevel WebSocket Library

HighLevel provides a simple, high-performance WebSocket library built on top of hioload-ws primitives. It maintains all the performance benefits (zero-copy, NUMA-aware, batch processing, high load) while providing an easy-to-use API.

## Quick Start

### Server

```go
package main

import (
    "log"
    "github.com/momentics/hioload-ws/highlevel"
)

func main() {
    server := highlevel.NewServer(":8080")

    server.HandleFunc("/echo", func(conn *highlevel.Conn) {
        defer conn.Close()

        for {
            mt, message, err := conn.ReadMessage()
            if err != nil {
                break
            }

            conn.WriteMessage(mt, message)
        }
    })

    log.Println("Starting server on :8080")
    server.ListenAndServe()
}
```

### Client

```go
conn, err := highlevel.Dial("ws://localhost:8080/echo")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Send a message
err = conn.WriteMessage(highlevel.TextMessage, []byte("Hello"))
if err != nil {
    log.Fatal(err)
}

// Read a message
mt, message, err := conn.ReadMessage()
if err != nil {
    log.Fatal(err)
}

// Send JSON
err = conn.WriteJSON(map[string]string{"key": "value"})
if err != nil {
    log.Fatal(err)
}
```

## Key Features Preserved

- **Zero Copy**: Automatic buffer management with hioload-ws pools
- **NUMA-aware**: Configuration options for NUMA node selection
- **Batch Processing**: Configurable batch sizes for optimal throughput
- **High Load**: Connection limits and optimized resource handling

## API Functions

### Server Functions
- `NewServer(addr)` - Create a new WebSocket server
- `HandleFunc(pattern, handler)` - Register a handler for a path
- `ListenAndServe()` - Start the server
- `Shutdown()` - Gracefully shut down the server

### Connection Functions  
- `ReadMessage()` - Read a WebSocket message
- `WriteMessage(type, data)` - Write a WebSocket message
- `ReadJSON(v)` - Read and unmarshal JSON
- `WriteJSON(v)` - Marshal and write JSON
- `Close()` - Close the connection

### Client Functions
- `Dial(url)` - Connect to a WebSocket server
- `DialWithOptions(url, opts...)` - Connect with options

## Configuration Options

### Server Options
- `WithMaxConnections(n)` - Set max concurrent connections
- `WithBatchSize(n)` - Set batch processing size
- `WithNUMANode(node)` - Set preferred NUMA node
- `WithChannelCapacity(n)` - Set internal channel capacity

### Client Options  
- `WithClientReadLimit(limit)` - Set read limit
- `WithClientDialTimeout(timeout)` - Set dial timeout
- `WithClientNUMANode(node)` - Set NUMA node
- `WithClientBatchSize(size)` - Set batch size