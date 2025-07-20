# Broadcast / Chat Server Example

## Table of Contents

1. [Introduction](#introduction)
2. [Features Demonstrated](#features-demonstrated)
3. [Prerequisites](#prerequisites)
4. [Installation and Build](#installation-and-build)
5. [Configuration Options](#configuration-options)
6. [Running the Broadcast Server](#running-the-broadcast-server)
7. [Architecture and Components](#architecture-and-components)
    - [Facade Orchestration](#facade-orchestration)
    - [Transport and Zero-Copy IO](#transport-and-zero-copy-io)
    - [NUMA-Aware Buffer Pool](#numa-aware-buffer-pool)
    - [Event Loop and Polling](#event-loop-and-polling)
    - [Executor and Work-Stealing](#executor-and-work-stealing)
    - [Session \& Connection Management](#session--connection-management)
    - [WebSocket Protocol Handling](#websocket-protocol-handling)
    - [Adapters \& Middleware](#adapters--middleware)
    - [Debug Probes \& Metrics](#debug-probes--metrics)
    - [CPU/NUMA Affinity](#cpu--numa-affinity)
8. [Code Walkthrough](#code-walkthrough)
9. [Runtime Behavior and Logging](#runtime-behavior-and-logging)
10. [Extending to Chat Rooms and Private Messaging](#extending-to-chat-rooms-and-private-messaging)
11. [Hot Reload and Dynamic Configuration](#hot-reload-and-dynamic-configuration)
12. [Observability and Health Checks](#observability-and-health-checks)
13. [Graceful Shutdown](#graceful-shutdown)
14. [Performance Tuning](#performance-tuning)
15. [Troubleshooting](#troubleshooting)
16. [FAQ](#faq)
17. [License \& Acknowledgements](#license--acknowledgements)

## Introduction

The **Broadcast / Chat Server** example demonstrates a one-to-many messaging scenario using the **hioload-ws** library. Clients connect via WebSocket and send binary or text messages. The server broadcasts each received message to **all** connected clients in a zero-copy, batched fashion, enabling real-time chat functionality with minimal latency and maximal throughput.

This example builds upon the standard Echo server and extends it to:

- **Maintain a live registry** of active WebSocket connections.
- **Broadcast messages** concurrently to many clients.
- **Leverage session management** for connection tracking.
- **Showcase high-performance aspects** of the library: NUMA-aware pooling, affinity, batch polling, and work-stealing.


## Features Demonstrated

1. **Connection Registry**: Track clients by unique identifiers (remote address).
2. **Efficient Broadcast**: Zero-copy broadcasting via pooled buffers and batched transport sends.
3. **Middleware Chain**: Logging, panic recovery, and metrics applied to broadcast handler.
4. **Concurrent Handler**: Thread-safe map of connections with `sync.RWMutex`.
5. **Facade \& Control**: Register `"active_clients"` debug probe showing current client count.
6. **Session Sharding**: Efficient session management for large numbers of connections.
7. **Affinity**: Pin reactor and executor threads for NUMA locality.
8. **Cross-Platform \& DPDK**: Support standard socket transport and DPDK with build tags.
9. **Graceful Cleanup**: Remove clients on disconnect and shutdown cleanly.
10. **Observability**: Log broadcast counts and errors, expose metrics via `Control.Stats()`.

## Prerequisites

- Go 1.23 or later installed.
- Linux (6.20+) or Windows Server 2016+ on amd64 architecture.
- Optional DPDK libraries for `-tags=dpdk` builds.
- Recommended `websocat` or custom WebSocket client for testing.


## Installation and Build

1. **Clone the repository:**

```bash
git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws
```

2. **Resolve dependencies:**

```bash
go mod tidy
```

3. **Build the broadcast example:**

```bash
go build -o broadcast-server ./examples/broadcast
```

4. **DPDK-enabled build (optional):**

```bash
go build -tags=dpdk -o broadcast-server ./examples/broadcast
```


## Configuration Options

The example accepts command-line flags mirroring `facade.Config`:

- `-addr` (string): Server HTTP listen address (default `:8080`).
- `-dpdk` (bool): Use DPDK transport if available (default `false`).
- `-shards` (int): Number of session shards (default `16`).
- `-workers` (int): Number of executor workers (default `4`).
- `-batch` (int): Poller batch size (default `16`).
- `-ring` (int): Poller ring capacity (default `1024`).
- `-numa` (int): Preferred NUMA node for allocations/affinity (default `-1`).


## Running the Broadcast Server

1. **Start the server:**

```bash
./broadcast-server -addr=":8080" -workers=8 -shards=32
```

Output:

```
Broadcast server listening on :8080 (DPDK=false)
```

2. **Connect multiple clients:**
    - **Client A:**

```bash
websocat ws://localhost:8080/ws
```

    - **Client B:**

```bash
websocat ws://localhost:8080/ws
```

3. **Send messages from any client.**
Each message is delivered to **all** connected clients.
4. **Live debug probe:**
    - The server registers `"active_clients"` probe.
    - Through the `Control.Stats()` API or logs, you can monitor the current client count.
5. **Shutdown:**
Press `Ctrl+C`. All connections are closed, and resources are released gracefully.

## Architecture and Components

### Facade Orchestration

- **`facade.HioloadWS`** initializes all internal modules: transport, pool manager, session manager, executor, poller, affinity, and control.
- Facilitates dynamic config and debug probe registration.


### Transport and Zero-Copy IO

- Standard socket transport uses epoll on Linux or IOCP on Windows for batch sends/receives.
- DPDK transport available with `-tags=dpdk`.


### NUMA-Aware Buffer Pool

- `pool.BufferPoolManager` provides buffers from per-node pools.
- Linux uses hugepages (`MAP_HUGETLB`) for large buffers.
- Windows uses `VirtualAlloc` with `MEM_LARGE_PAGES` if available.


### Event Loop and Polling

- `concurrency.EventLoop` drives batched event dispatch.
- Handlers registered via `adapters.PollerAdapter` receive incoming data as `interface{}` events.


### Executor and Work-Stealing

- `concurrency.Executor` runs background tasks and can be used to offload heavy work in handlers.
- NUMA-aware worker pinning reduces cross-node traffic.


### Session \& Connection Management

- `session.SessionManager` shards sessions for quick lookups and scaled concurrency.
- `protocol.WSConnection` wraps transport for frame parsing, buffer management, and handler callbacks.


### WebSocket Protocol Handling

- `protocol.UpgradeToWebSocket` performs handshake.
- `WSConnection` handles `Recv()` → `[]api.Buffer` and `Send(frame)` with zero-copy.


### Adapters \& Middleware

- `LoggingMiddleware`, `RecoveryMiddleware`, and `MetricsMiddleware` chain around `ChatHandler`.
- `ChatHandler` implements `api.Handler`.


### Debug Probes \& Metrics

- `GetControl().RegisterDebugProbe("active_clients", ...)` captures live client count.
- `MetricsMiddleware` increments per-message metrics.


### CPU/NUMA Affinity

- `affinity.PinCurrentThread` binds reactor and executor threads to specified cores and NUMA nodes for improved locality.


## Code Walkthrough

### main.go

- Parse flags.
- Create and configure `HioloadWS`.
- Initialize `ChatHandler` and register debug probe.
- Start facade services.
- Build middleware chain and register with poller.
- HTTP `/ws` handler:
    - Upgrade HTTP to WebSocket.
    - Create `WSConnection`, set handler, start loops.
    - Add client to registry, spawn goroutine to remove on `Done()`.
- Wait on OS signals and call `hioload.Stop()`.


### ChatHandler

```go
type ChatHandler struct {
    mu       sync.RWMutex
    sessions map[string]*protocol.WSConnection
    hioload  *facade.HioloadWS
}

func (h *ChatHandler) Handle(data any) error {
    buf, ok := data.([]byte)
    if !ok {
        return nil
    }
    h.mu.RLock()
    defer h.mu.RUnlock()
    for id, conn := range h.sessions {
        out := h.hioload.GetBuffer(len(buf))
        copy(out.Bytes(), buf)
        conn.Send(&protocol.WSFrame{IsFinal: true, Opcode: protocol.OpcodeBinary, Payload: out.Bytes()})
        out.Release()
    }
    log.Printf("[broadcast] message sent to %d clients", len(h.sessions))
    return nil
}
```


## Runtime Behavior and Logging

- Upon each client connect/disconnect, logs:

```
[chat] client connected: <addr>
[chat] client disconnected: <addr>
```

- Each broadcast logs number of clients:

```
[broadcast] message sent to 42 clients
```

- Any send errors are logged with client identifier.


## Extending to Chat Rooms and Private Messaging

- **Chat Rooms:** Use `session.Context()` to tag each connection with room IDs.
- **Private Messaging:** Add direct mapping `map[string]string` from user IDs to connections.
- **Presence:** Implement heartbeat or ping/pong handlers to mark active/inactive clients.


## Hot Reload and Dynamic Configuration

- Register hooks via `GetControl().OnReload(func())` to react to config changes.
- E.g., adjust executor worker count or poller batch size at runtime:

```go
hioload.GetControl().OnReload(func() {
    newWorkers := hioload.GetControl().GetConfig()["num_workers"].(int)
    hioload.Submit(func() {
        hioload.ResizeExecutor(newWorkers)
    })
})
```


## Observability and Health Checks

- Expose `/metrics` HTTP endpoint to return `json.Marshal(hioload.GetControl().Stats())`.
- Expose `/debug` to return `json.Marshal(hioload.GetControl().RegisterDebugProbe())` output.
- Integrate with Prometheus or external monitoring systems.


## Graceful Shutdown

- The server catches `SIGINT` and `SIGTERM`.
- On shutdown: calls `PollerAdapter.Stop()`, `transport.Close()`, `Executor.Close()`, and `Affinity.Unpin()`.
- Connections close naturally via `WSConnection.Done()` channels.


## Performance Tuning

- Increase `workers` and `shards` for higher concurrency.
- Tune `batch` and `ring` sizes to match expected message rates and sizes.
- Use DPDK transport for kernel-bypass performance when needed.
- Pin hot loops (event poller and executor) to dedicated cores.
- Monitor metrics and debug probes to identify contention points.


## Troubleshooting

- **Connections Drop on High Load:** Increase poller ring capacity and worker count.
- **High Latency:** Verify NUMA affinity, reduce GC by maximizing zero-copy paths.
- **DPDK Fallback:** Check build tags and DPDK library versions.
- **Windows Affinity Failures:** Ensure proper privileges for thread affinity API calls.


## FAQ

**Q:** How many clients can this server handle?
A: Millions of connections theoretically; practical limit depends on OS network stack and hardware.

**Q:** Is message order preserved?
A: Yes; each broadcast iterates snapshot of `sessions` map under read lock.

**Q:** Can I restrict broadcasts to specific subsets?
A: Implement filtering logic in `ChatHandler.Handle`.

**Q:** What if a client is slow?
A: `WSConnection.Send` returns error if outbox is full; handler can drop or cleanup slow clients.

## License \& Acknowledgements

- Licensed under Apache License 2.0.
- Thanks to the Go community, DPDK, and OS-specific IO abstractions for foundational primitives.
- See [LICENSE](../../LICENSE) and [NOTICE](../../NOTICE) files in the project root for full third-party license details.
