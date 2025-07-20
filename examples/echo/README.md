# Echo WebSocket Server Example

## Table of Contents

1. [Overview](#overview)
2. [Features Demonstrated](#features-demonstrated)
3. [Prerequisites](#prerequisites)
4. [Installation and Build](#installation-and-build)
5. [Configuration Options](#configuration-options)
6. [Running the Echo Server](#running-the-echo-server)
7. [Architecture and Components](#architecture-and-components)
    - [Facade Layer](#facade-layer)
    - [Transport Layer](#transport-layer)
    - [Buffer Pool](#buffer-pool)
    - [Event Loop (Poller)](#event-loop-poller)
    - [Executor (Thread Pool)](#executor-thread-pool)
    - [Session Management](#session-management)
    - [WebSocket Protocol](#websocket-protocol)
    - [Adapters and Middleware](#adapters-and-middleware)
    - [Control and Debug Probes](#control-and-debug-probes)
    - [Affinity and NUMA Awareness](#affinity-and-numa-awareness)
8. [Code Walkthrough](#code-walkthrough)
9. [Hot Reload and Debug Probes](#hot-reload-and-debug-probes)
10. [Metrics and Observability](#metrics-and-observability)
11. [Graceful Shutdown](#graceful-shutdown)
12. [Extending the Example](#extending-the-example)
13. [Troubleshooting](#troubleshooting)
14. [Frequently Asked Questions (FAQ)](#frequently-asked-questions)
15. [Acknowledgements and License](#acknowledgements-and-license)

## Overview

The **Echo WebSocket Server** example is a minimal, yet comprehensive demonstration of the **hioload-ws** library’s core capabilities. It implements a WebSocket server that echoes any message back to the sender, leveraging:

- **Zero-copy, batched IO**
- **NUMA-aware buffer pooling**
- **Cross-platform transport** (TCP by default; optional DPDK with build tag)
- **Lock-free data structures and batch-oriented event loops**
- **Extensible middleware chain** (logging, panic recovery, metrics)
- **Runtime configuration and hot-reloadable debug probes**
- **CPU/NUMA affinity pinning for extreme performance**
- **Graceful shutdown handling**

The example is fully cross-platform: it compiles and runs on Linux (epoll-based transport) and Windows (IOCP-based transport) as well as on any amd64 environment. With the `-tags=dpdk` flag and appropriate DPDK bindings installed, it can also use a DPDK-backed transport for kernel-bypass performance.

## Features Demonstrated

1. **Facade Pattern**: Unified entry point (`facade.HioloadWS`) for all library components.
2. **Configurable Transport**: Switch between standard socket transport and DPDK via configuration.
3. **NUMA-aware Buffer Pool**: Allocate and reuse memory from per-NUMA-node pools with minimal garbage collection overhead.
4. **Batched Poll-Mode Event Loop**: High-throughput event dispatch using lock-free rings and adaptive backoff.
5. **Executor (Work-Stealing Thread Pool)**: Running tasks in parallel across worker goroutines, with NUMA affinity.
6. **Session Management**: Tracking active WebSocket sessions for diagnostics.
7. **WebSocket Protocol**: RFC6455-compliant frame encoding/decoding, zero-copy payload handling.
8. **Adapters and Middleware**: Plug-and-play logging, panic recovery, metrics middleware.
9. **Control API**: Dynamic configuration, hot-reload hooks, debug probes for live introspection.
10. **Affinity**: Pin reactor and executor threads to CPU cores and NUMA nodes.
11. **Graceful Shutdown**: Catch OS signals and cleanly close all connections and resources.
12. **Observability**: Live runtime metrics, debug probe outputs.

## Prerequisites

- Go toolchain **1.23** or later
- **Linux** (kernel 6.20+ recommended) or **Windows Server 2016+** on **amd64**
- Optional: DPDK libraries and Go bindings (e.g., `github.com/yerden/go-dpdk`) for `-tags=dpdk` build
- `git`, `make` (optional)


## Installation and Build

1. **Clone the repository:**

```bash
git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws
```

2. **Download dependencies:**

```bash
go mod tidy
```

3. **Build the example:**

```bash
go build -o echo-server ./examples/echo
```

    - To enable DPDK transport, install Go DPDK bindings and run:

```bash
go build -tags=dpdk -o echo-server ./examples/echo
```


## Configuration Options

The example supports several command-line flags that map to the `facade.Config` fields:

- `-addr` (string): Listen address (default `:8080`).
- `-dpdk` (bool): Enable DPDK transport if built with `-tags=dpdk` (default `false`).
- `-shards` (int): Number of session shards for concurrency (default `16`).
- `-workers` (int): Number of executor workers (default `4`).
- `-batch` (int): Poller batch size (default `16`).
- `-ring` (int): Poller ring buffer capacity (default `1024`).
- `-numa` (int): Preferred NUMA node for buffer allocations and thread pinning (default `-1` = any).
- `-debug` (bool): Enable debug probes output (default `true`).
- `-metrics` (bool): Enable metrics collection (default `true`).


### Example:

```bash
./echo-server -addr=":9001" -workers=8 -shards=32 -numa=0
```


## Running the Echo Server

1. **Start the server:**

```bash
./echo-server -addr=":8080"
```

Output:

```
Server listening on :8080 (DPDK=false)
```

2. **Connect a WebSocket client:**
    - **Browser-based:** Open console and run:

```js
let ws = new WebSocket("ws://localhost:8080/ws");
ws.onmessage = evt => console.log("Echoed:", evt.data);
ws.onopen = () => ws.send("Hello, hioload-ws!");
```

    - **Command-line:**

```bash
websocat ws://localhost:8080/ws
```

3. **Send messages:**
All sent messages will be echoed back verbatim.
4. **View runtime stats:**
The server logs metrics and you can integrate with CLI or HTTP endpoint (if implemented).
5. **Stop the server:**
Press `Ctrl+C`. The server will gracefully close all connections and exit.

## Architecture and Components

### Facade Layer

- `facade.HioloadWS` is the central orchestrator. It initializes:
    - **Transport** (`api.Transport`): either standard or DPDK-backed.
    - **Buffer Pool Manager**: NUMA-aware pools.
    - **Session Manager**: Sharded storage for active sessions.
    - **Executor**: NUMA-aware thread pool.
    - **Poller**: Batched event-driven loop.
    - **Affinity**: CPU/NUMA pinning logic.
    - **Control**: Configuration and debug API.


### Transport Layer

- **Linux**: `internal/transport/transport_linux.go` uses `unix.SendmsgBuffers`/`RecvmsgBuffers` for zero-copy batched IO.
- **Windows**: `internal/transport/transport_windows.go` leverages IOCP and `WSASend`/`WSARecv` for high-throughput.
- **DPDK**: `internal/transport/dpdk_transport.go` stub that can be replaced by real Go DPDK bindings when built with `-tags=dpdk`.


### Buffer Pool

- `pool.BufferPoolManager` provides per-NUMA-node `BufferPool` instances.
- Linux buffer pools allocate hugepages via `mmap(HUGETLB)` and fallback to `make([]byte)`.
- Windows buffer pools use `VirtualAlloc` with `MEM_LARGE_PAGES`.


### Event Loop (Poller)

- `concurrency.EventLoop` is a batched, lock-free, adaptive backoff loop.
- `adapters.PollerAdapter` wraps `EventLoop` and converts `api.Handler` to `EventHandler`.


### Executor (Thread Pool)

- `concurrency.Executor` manages workers with local lock-free queues and a global queue.
- Implements work-stealing, NUMA-aware affinity, and statistics tracking.


### Session Management

- `session.SessionManager` shards sessions by FNV-1a hash.
- `session.Context` (`context_store.go`) provides key-value storage with TTL and propagation flags.


### WebSocket Protocol

- `protocol.UpgradeToWebSocket` performs HTTP handshake.
- `protocol.WSConnection` manages framing, recv/send loops, zero-copy buffer integration.
- Frame codec (`frame.go`) implements RFC6455 with zero-copy decode into pooled buffers.


### Adapters and Middleware

- `adapters.HandlerFunc`, `MiddlewareHandler` allow chaining:
    - `LoggingMiddleware` – logs message processing.
    - `RecoveryMiddleware` – recovers from panics.
    - `MetricsMiddleware` – updates `handler.processed` metric in `Control`.


### Control and Debug Probes

- `api.Control` exposes `GetConfig`, `SetConfig`, `Stats`, `OnReload`, and `RegisterDebugProbe`.
- `adapters.ControlAdapter` connects to `control.ConfigStore`, `control.MetricsRegistry`, `control.DebugProbes`.
- Example registers `"active_sessions"` probe, showing live connection count.


### Affinity and NUMA Awareness

- `api.Affinity` lets you pin/unpin threads to CPU cores or NUMA nodes.
- `adapters.AffinityAdapter` implements `Pin` and `Unpin` using `internal/concurrency`.


## Code Walkthrough

### main.go

- Parse flags.
- Build and customize `facade.Config`.
- Create `HioloadWS`.
- Register debug probe:

```go
hioload.GetControl().RegisterDebugProbe("active_sessions", func() any {
    return fmt.Sprintf("%d", hioload.GetSessionCount())
})
```

- Start facade: `hioload.Start()`.
- Build middleware chain: Logging → Recovery → Metrics.
- Register the middleware handler: `hioload.RegisterHandler(mw)`.
- HTTP handler `/ws`: Upgrade to WebSocket, create `WSConnection`, set handler, start, log client.
- Wait for `SIGINT`/`SIGTERM`, then call `hioload.Stop()`.


## Hot Reload and Debug Probes

- **OnReload**: You can attach functions to be called when `SetConfig` is invoked on `Control`.
- **Debug Probes**: `control.DebugProbes` aggregates named functions.
- Example `"active_sessions"` prints number of active sessions.
- Additional probes like `"platform.cpus"`, `"handler.processed"`, etc., are registered by `ControlAdapter`.


## Metrics and Observability

- `MetricsMiddleware` updates `handler.processed` counter on each message.
- `control.MetricsRegistry` stores arbitrary key/value metrics.
- `ControlAdapter.Stats()` merges metrics and debug probe data.
- You can periodically log or expose these stats via HTTP for external monitoring.


## Graceful Shutdown

- Capture OS signals (`SIGINT`, `SIGTERM`).
- On shutdown:
    - Stop poller (`PollerAdapter.Stop()`).
    - Close transport.
    - Close executor.
    - Unpin CPU affinity.
- All `WSConnection` loops observe `transport.Recv` errors or `Close()` to terminate.


## Extending the Example

- **Broadcast Server:** Build on echo example by tracking all connections in a `map[string]*WSConnection` and broadcasting messages instead of echo.
- **Chat Rooms:** Use `session.Context` to store room membership and route messages selectively.
- **Scheduled Tasks:** Incorporate `internal/concurrency.TaskScheduler` for periodic broadcasts or heartbeats.
- **Protocol Extensions:** Add fragmentation, compression (permessage-deflate), or custom opcodes.
- **REST Admin API:** Expose `/metrics` and `/debug` endpoints to retrieve `Control.Stats()` and `DebugProbes.DumpState()` JSON.
- **Hot Reload Tests:** Create an HTTP endpoint to call `Control.SetConfig()` and trigger live reload hooks.


## Troubleshooting

- **Missing DPDK Transport:** Ensure you build with `-tags=dpdk` and install Go DPDK bindings.
- **Affinity Errors on Windows:** CPU pinning on Windows may require elevated privileges.
- **Buffer Pool Exhaustion:** Adjust per-node channel capacity in `bufferpool_linux.go` if you experience allocation stalls.
- **Large Frame Errors:** The default ring and batch sizes (`BatchSize=16`, `RingCapacity=1024`) can be tuned via config flags.
- **Unhandled Panics in Handlers:** Ensure you include `RecoveryMiddleware` to avoid crashing event loop.


## Frequently Asked Questions

**Q: Why zero-copy?**
Zero-copy buffer management minimizes GC pauses and CPU cycles by reusing memory regions for incoming and outgoing frames, crucial under high load.

**Q: How does NUMA-awareness help?**
Allocating buffers and pinning threads on the same NUMA node reduces cross-node memory traffic latency and improves cache locality.

**Q: Can I run on Windows?**
Yes; the example supports IOCP-based transport on Windows with identical code and flags.

**Q: What happens if DPDK init fails?**
The facade falls back to standard transport automatically and logs a warning:

```
[facade] DPDK unavailable or init failed: <error>, fallback to native transport
```

**Q: How to adjust performance parameters?**
Use command-line flags to tune workers, batch size, ring capacity, NUMA node, and debug/metrics toggles.

## Acknowledgements and License

- **hioload-ws** is licensed under the Apache License 2.0.
- Contributions from the Go community, DPDK, and OS-specific IO primitives.
- See [LICENSE](../../LICENSE) and [NOTICE](../../NOTICE) files in the project root for full third-party license details.

