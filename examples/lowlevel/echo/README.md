# Hioload-WS Echo Example

This package provides a comprehensive **Echo WebSocket Server** example built on top of the `hioload-ws` library. demonstrates how to build a high-performance, zero-copy, NUMA-aware WebSocket echo server in Go, leveraging the full power of `hioload-ws`’s batch I/O, lock-free data structures, and affinity control. The example is designed for advanced Go and systems programmers exploring extreme-performance networking for real-world applications.

## Table of Contents

1. [Overview](#overview)  
2. [Key Features](#key-features)  
3. [Prerequisites](#prerequisites)  
4. [Installation](#installation)  
5. [Configuration Options](#configuration-options)  
6. [Architecture and Design](#architecture-and-design)  
   6.1 [Facade Pattern](#facade-pattern)  
   6.2 [Middleware Chain](#middleware-chain)  
   6.3 [Zero-Copy Buffers](#zero-copy-buffers)  
   6.4 [Batch I/O Reactor](#batch-io-reactor)  
   6.5 [NUMA-Aware Affinity](#numa-aware-affinity)  
   6.6 [Lock-Free Structures](#lock-free-structures)  
7. [Code Walkthrough](#code-walkthrough)  
   7.1 [main.go](#maingo)  
   7.2 [go.mod & go.sum](#gomod--gosum)  
8. [Running the Example](#running-the-example)  
9. [Performance Considerations](#performance-considerations)  
10. [Testing and Validation](#testing-and-validation)  
11. [Extending the Example](#extending-the-example)  
12. [Debugging and Metrics](#debugging-and-metrics)  
13. [Graceful Shutdown](#graceful-shutdown)  
14. [License](#license)  
15. [Contact & Contributions](#contact--contributions)  

---

## Overview

The Echo example demonstrates how to:

- Initialize and configure a `hioload-ws` **Server** facade.  
- Use a **zero-copy** NUMA-aware buffer pool.  
- Set up a **batch-mode** reactor (poll-loop) for maximum throughput.  
- Implement a simple **echo handler** that reads incoming messages and replies with the same payload.  
- Integrate **middleware** for logging, panic recovery, and message-count metrics.  
- Pin threads and reactors to NUMA nodes and CPU cores for low-latency processing.  
- Manage graceful startup and shutdown under heavy load.  

This example is targeted at performance-oriented Go developers, systems programmers, and architects building latency-sensitive WebSocket backends.

---

## Key Features

- **Zero-Copy**: Buffers are allocated from a NUMA-aware pool, avoiding allocations and copying.  
- **Batch I/O Reactor**: Uses `PollerAdapter` to process up to N events per tick in a lock-free ring.  
- **NUMA-Aware Affinity**: Threads and executors are pinned to specific NUMA nodes/CPUs to minimize cross-node traffic.  
- **Lock-Free Data Structures**: Internally uses lock-free ring buffers and queues for minimal contention.  
- **Middleware Chain**: Composable logging, recovery, and metrics via the `adapters` package.  
- **Graceful Shutdown**: Clean teardown on SIGINT/SIGTERM, ensuring no data loss.  
- **Cross-Platform**: Works on Linux (epoll, io_uring) and Windows (IOCP).  
- **Extensible**: Showcases clear extension points for custom transports, protocols, and handlers.  

---

## Prerequisites

- Go 1.23 or later  
- Linux kernel ≥ 6.20 or Windows Server 2016+ on amd64  
- `hioload-ws` library (module `github.com/momentics/hioload-ws`) accessible via Go modules  
- Basic familiarity with Go, goroutines, and networking concepts  

---

## Installation

Clone the repository and navigate to the example directory:

```

git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws/examples/lowlevel/echo
go mod tidy

```

---

## Configuration Options

The example exposes several command-line flags for tuning:

| Flag           | Default  | Description                                                                           |
|----------------|----------|---------------------------------------------------------------------------------------|
| `-addr`        | `:9001`  | TCP address for WebSocket listener                                                    |
| `-batch`       | `32`     | Number of events processed per reactor poll loop                                      |
| `-ring`        | `1024`   | Capacity of internal ring buffer                                                      |
| `-workers`     | `0`      | Number of executor goroutines (0 = runtime.NumCPU())                                  |
| `-numa`        | `-1`     | Preferred NUMA node for buffer allocation and thread pinning (-1 = auto-detect)      |

Example:

```

go run main.go -addr=":9001" -batch=64 -ring=2048 -workers=8 -numa=0

```

---

## Architecture and Design

### Facade Pattern

`server.NewServer(cfg, opts...)` constructs the unified Server facade, initializing:

1. **ControlAdapter** for hot-reload and debug probes.  
2. **BufferPoolManager** for NUMA-aware zero-copy buffers.  
3. **WebSocketListener** for TCP accept and handshake.  
4. **PollerAdapter** as the reactor loop.  
5. **ExecutorAdapter** as the worker pool.  

This facade hides internal complexities and provides a minimal API:

```

srv, err := server.NewServer(cfg, server.WithMiddleware(...))

```

### Middleware Chain

Middleware functions wrap the base `api.Handler` to add cross-cutting concerns:

- `LoggingMiddleware`: logs data types and errors.  
- `RecoveryMiddleware`: recovers from panics in handler code.  
- `HandlerMetricsMiddleware`: increments `handler.processed` counter on each call.

Assembling:

```

srv.UseMiddleware(
adapters.LoggingMiddleware,
adapters.RecoveryMiddleware,
adapters.HandlerMetricsMiddleware(srv.GetControl()),
)

```

### Zero-Copy Buffers

Buffers are allocated from a **slab pool** backed by huge pages or VirtualAllocExNuma:

- Pooled by fixed size classes (e.g., 64 KiB).  
- Returned to pool via `Buffer.Release()`.  
- NUMA-aware allocation for low-latency access.

### Batch I/O Reactor

`PollerAdapter` uses an internal `EventLoop` to:

- Batch up to `BatchSize` events from a lock-free channel.  
- Dispatch each event to registered handlers without locks.  
- Adaptive backoff when no events are available.

### NUMA-Aware Affinity

On startup, `Server.Run` pins the reactor thread to the specified NUMA node:

```

aff := adapters.NewAffinityAdapter()
aff.Pin(-1, cfg.NUMANode)
defer aff.Unpin()

```

Executor workers are also pinned per NUMA node for consistent memory locality.

### Lock-Free Structures

- **RingBuffer**: concurrent FIFO for passing events.  
- **LockFreeQueue**: single-producer/single-consumer task queues.  
- **SlabPool**: lock-free stack of free buffers.

All critical paths avoid mutexes and context switches where possible.

---

## Code Walkthrough

### main.go

```

package main

import (
"flag"
"fmt"
"os"
"os/signal"
"sync/atomic"
"syscall"
"time"

    "github.com/momentics/hioload-ws/adapters"
    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/core/protocol"
    "github.com/momentics/hioload-ws/lowlevel/server"
    )

func main() {
// Parse flags
addr := flag.String("addr", ":9001", "WebSocket listen address")
batch := flag.Int("batch", 32, "Reactor batch size")
ring := flag.Int("ring", 1024, "Reactor ring capacity")
workers := flag.Int("workers", 0, "Executor worker count (0 = NumCPU)")
numa := flag.Int("numa", -1, "Preferred NUMA node (-1 = auto)")
flag.Parse()

    // Build server configuration
    cfg := server.DefaultConfig()
    cfg.ListenAddr = *addr
    cfg.BatchSize = *batch
    cfg.ReactorRing = *ring
    if *workers > 0 {
        cfg.ExecutorWorkers = *workers
    }
    cfg.NUMANode = *numa
    
    // Initialize server with middleware
    srv, err := server.NewServer(
        cfg,
        server.WithMiddleware(
            adapters.LoggingMiddleware,
            adapters.RecoveryMiddleware,
        ),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "NewServer error: %v\n", err)
        os.Exit(1)
    }
    
    // Add metrics middleware
    srv.UseMiddleware(adapters.HandlerMetricsMiddleware(srv.GetControl()))
    
    // Debug probes
    var activeConns int64
    var totalMsgs int64
    ctrl := srv.GetControl()
    ctrl.RegisterDebugProbe("active_connections", func() any { return atomic.LoadInt64(&activeConns) })
    ctrl.RegisterDebugProbe("messages_processed", func() any { return atomic.LoadInt64(&totalMsgs) })
    
    // Periodic stats output
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            stats := ctrl.Stats()
            fmt.Printf("[%s] Active: %v, Msgs: %v\n",
                time.Now().Format(time.Stamp), stats["active_connections"], stats["messages_processed"])
        }
    }()
    
    // Echo handler
    echoHandler := adapters.HandlerFunc(func(data any) error {
        buf := data.(api.Buffer)
        atomic.AddInt64(&totalMsgs, 1)
        conn := api.FromContext(api.ContextFromData(data)).(*protocol.WSConnection)
    
        frame := &protocol.WSFrame{
            IsFinal:    true,
            Opcode:     protocol.OpcodeBinary,
            PayloadLen: int64(len(buf.Bytes())),
            Payload:    buf.Bytes(),
        }
        buf.Release()
        return conn.SendFrame(frame)
    })
    
    // Connection tracking middleware
    track := func(next api.Handler) api.Handler {
        return adapters.HandlerFunc(func(data any) error {
            switch evt := data.(type) {
            case api.OpenEvent:
                atomic.AddInt64(&activeConns, 1)
                defer next.Handle(evt)
            case api.CloseEvent:
                atomic.AddInt64(&activeConns, -1)
                defer next.Handle(evt)
            default:
                return next.Handle(data)
            }
            return nil
        })
    }
    srv.UseMiddleware(track)
    
    // Run server
    go func() {
        if err := srv.Run(echoHandler); err != nil {
            fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
            os.Exit(1)
        }
    }()
    
    // Graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    fmt.Println("Shutting down echo server...")
    srv.Shutdown()
    fmt.Println("Server stopped.")
    }

```

### go.mod & go.sum

```

module github.com/momentics/hioload-ws/examples/lowlevel/echo

go 1.23

require github.com/momentics/hioload-ws v0.0.0

```

---

## Running the Example

Build and run:

```

cd examples/lowlevel/echo
go run main.go

```

Connect with any WebSocket client:

```

wscat -c ws://localhost:9001
> Hello
< Hello

```

---

## Performance Considerations

- **Batch Size** and **Ring Capacity** significantly affect throughput vs. latency trade-offs. Tune based on workload.  
- **NUMA Node** pinning reduces cross-node memory access.  
- **Executor Workers** should match CPU cores for optimal utilization.  
- **Zero-Copy** and **lock-free** avoid GC pressure and context switches.  
- Benchmarks with `k6`, `wrk`, or custom load tests recommended.

---

## Testing and Validation

- Unit tests in `internal/` and `api/` ensure interface compliance and correctness.  
- Integration tests simulate echo behavior under load.  
- Use `go test -bench ./...` to run benchmarks.

---

## Debugging and Metrics

- Register custom debug probes via `srv.GetControl().RegisterDebugProbe(name, fn)`.  
- Expose metrics via HTTP or integrate with Prometheus by exporting `Control.Stats()`.  
- Use `pprof` or `trace` for performance profiling.

---

## Graceful Shutdown

The example handles SIGINT/SIGTERM to:

1. Close listener to stop accepting new connections.  
2. Signal reactor to stop polling.  
3. Wait up to `ShutdownTimeout` for in-flight events.  
4. Cleanly close all transports and executors.

This ensures no data loss and orderly resource release.

---

## License

This example is licensed under **Apache License 2.0**. See [LICENSE](../../LICENSE).

---

## Contact & Contributions

For questions, feature requests, or contributions:

- **Author**: momentics <momentics@gmail.com>  
- **Repository**: https://github.com/momentics/hioload-ws  
- **Issues & PRs**: Welcome!  

---
