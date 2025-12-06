# High-level API Examples

This directory contains examples demonstrating the usage of the hioload-ws high-level API.

## Available Examples

### Echo Server
An echo server that returns any message received back to the client.

**Usage:**
```bash
go run examples/highlevel/echo/main.go
```

**Options:**
- `-port`: Port to listen on (default: 8080)
- `-numa`: NUMA node to bind to (-1 for auto)
- `-batch`: Batch size for processing (default: 32)
- `-maxconn`: Maximum number of connections (default: 10000)

### WebSocket Client
A client that connects to the echo server and sends/receives messages.

**Usage:**
```bash
go run examples/highlevel/client/main.go
```

## Key Features Demonstrated

1. **Server Creation**: Creating and configuring a WebSocket server
2. **Route Handling**: Registering handlers for different paths
3. **Message Handling**: Reading and writing text/binary messages
4. **JSON Support**: Sending and receiving JSON messages
5. **Connection Management**: Proper connection lifecycle management
6. **Graceful Shutdown**: Proper server shutdown handling

## Performance Features

All examples utilize the high-performance features of hioload-ws:
- Zero-copy buffer management
- NUMA-aware memory allocation
- Batch processing
- Lock-free data structures
- Efficient memory pools