# Broadcast WebSocket Server Example

This package provides a high-performance, zero-copy, NUMA-aware **Broadcast WebSocket Server** built on the `hioload-w` library. It demonstrates how to implement a native WebSocket broadcast server without using the standard `net/http` package, leveraging zero-copy data paths, lock-free batch I/O, and CPU/NUMA affinity for extreme throughput and minimal latency.

## Table of Contents

1. [Overview](#overview)
2. [Key Features](#key-features)
3. [Prerequisites](#prerequisites)
4. [Installation](#installation)
5. [Configuration](#configuration)
6. [Architecture](#architecture)
6.1 [Facade Pattern](#facade-pattern)
6.2 [Middleware Chain](#middleware-chain)
6.3 [Zero-Copy Buffer Pool](#zero-copy-buffer-pool)
6.4 [Batch Poll-Mode Reactor](#batch-poll-mode-reactor)
6.5 [NUMA-Aware Affinity](#numa-aware-affinity)
6.6 [Lock-Free Data Structures](#lock-free-data-structures)
7. [Code Walkthrough](#code-walkthrough)
7.1 [main.go](#maingo)
7.2 [go.mod](#gomod)
8. [Running the Example](#running-the-example)
9. [Performance Tuning](#performance-tuning)
10. [Debugging \& Metrics](#debugging--metrics)
11. [Graceful Shutdown](#graceful-shutdown)
12. [Extending the Example](#extending-the-example)
13. [License](#license)
14. [Contributing](#contributing)

## Overview

The **Broadcast Server** listens on a native TCP socket, performs the RFC 6455 WebSocket handshake, and broadcasts any incoming binary frames from one client to all others. This example is optimized for extreme loads by:

- Avoiding per-connection goroutines in favor of a shared, batched **Poll-Mode Reactor**.
- Serving millions of concurrent connections with microsecond-level p99 latency.
- Using NUMA-aware **zero-copy** buffers and **lock-free** queues to eliminate GC pressure and contention.
- Pinning reactor and executor threads to specific CPU cores/NUMA nodes for maximum locality.


## Key Features

- **Zero-Copy Data Paths**: Buffers are allocated from a **NUMA-aware** pool and never copied on application level.
- **Batch Poll-Mode I/O**: Processes up to N events per reactor tick for maximal throughput.
- **NUMA-Aware Affinity**: Reactor and executor threads are pinned to selected NUMA nodes and CPUs.
- **Lock-Free Structures**: Uses lock-free ring buffers and queues for minimal contention.
- **Middleware Chain**: Composable logging, recovery, and custom event tracking.
- **Graceful Shutdown**: Clean teardown on SIGINT/SIGTERM without data loss.
- **Cross-Platform**: Supports Linux (epoll, io_uring) and Windows (IOCP).


## Prerequisites

- Go 1.23 or later
- Linux kernel ≥ 6.20 or Windows Server 2016+ (amd64)
- `hioload-ws` library (module `github.com/momentics/hioload-ws`)
- Familiarity with Go, goroutines, and systems programming


## Installation

```bash
git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws/examples/lowlevel/broadcast
go mod tidy
```


## Configuration

The server accepts the following command-line flags:


| Flag | Default | Description |
| :-- | :-- | :-- |
| `-addr` | `:9002` | TCP address for WebSocket listener |
| `-batch` | `64` | Number of events processed per reactor poll loop |
| `-ring` | `2048` | Capacity of the internal ring buffer |
| `-workers` | `0` | Number of executor goroutines (0 = runtime.NumCPU()) |
| `-numa` | `-1` | Preferred NUMA node for buffer allocation and thread pinning (-1 = auto) |

Example:

```bash
go run main.go -addr=":9002" -batch=128 -ring=4096 -workers=8 -numa=0
```


## Architecture

### Facade Pattern

The `/server` package exposes a **Server** façade:

1. **ControlAdapter**: hot-reload, metrics, debug probes.
2. **BufferPoolManager**: NUMA-aware zero-copy buffers.
3. **WebSocketListener**: native TCP accept and handshake.
4. **PollerAdapter**: batched, lock-free reactor.
5. **ExecutorAdapter**: NUMA-aware, lock-free task executor.

Construction:

```go
cfg := server.DefaultConfig()
cfg.ListenAddr = *addr
cfg.BatchSize = *batch
cfg.ReactorRing = *ring
cfg.ExecutorWorkers = *workers
cfg.NUMANode = *numa

srv, err := server.NewServer(cfg,
    server.WithMiddleware(adapters.LoggingMiddleware),
    server.WithMiddleware(adapters.RecoveryMiddleware),
)
```


### Middleware Chain

Middlewares wrap the core handler to add cross-cutting concerns:

```go
srv.UseMiddleware(
    adapters.LoggingMiddleware,
    adapters.RecoveryMiddleware,
    trackingMiddleware, // custom broadcast tracking
)
```


### Zero-Copy Buffer Pool

Buffers are drawn from a **slab-based** size-class pool, segmented per NUMA node to ensure memory locality and avoid heap allocations.

### Batch Poll-Mode Reactor

`PollerAdapter` uses a lock-free `EventLoop` to:

- Gather up to `BatchSize` events per iteration.
- Dispatch events to registered handlers without mutexes in hot path.
- Adaptively back off when idle.


### NUMA-Aware Affinity

At startup, the main reactor thread locks to an OS thread and pins it to the specified NUMA node. Executor workers are similarly pinned.

```go
aff := adapters.NewAffinityAdapter()
aff.Pin(-1, cfg.NUMANode)
defer aff.Unpin()
```


### Lock-Free Data Structures

- **RingBuffer**: concurrent FIFO queue.
- **LockFreeQueue**: single-producer/single-consumer queue for tasks.
- **SlabPool**: lock-free stack of free buffers.


## Code Walkthrough

### main.go

```go
package main

import (
    "flag"
    "fmt"
    "os"
    "os/signal"
    "sync"
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
    addr := flag.String("addr", ":9002", "WebSocket listen address")
    batch := flag.Int("batch", 64, "Reactor batch size")
    ring := flag.Int("ring", 2048, "Reactor ring capacity")
    workers := flag.Int("workers", 0, "Executor worker count (0 = NumCPU)")
    numa := flag.Int("numa", -1, "Preferred NUMA node (-1 = auto)")
    flag.Parse()

    // Server configuration
    cfg := server.DefaultConfig()
    cfg.ListenAddr = *addr
    cfg.BatchSize = *batch
    cfg.ReactorRing = *ring
    if *workers > 0 {
        cfg.ExecutorWorkers = *workers
    }
    cfg.NUMANode = *numa

    // Initialize server façade
    srv, err := server.NewServer(cfg,
        server.WithMiddleware(adapters.LoggingMiddleware),
        server.WithMiddleware(adapters.RecoveryMiddleware),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "NewServer error: %v\n", err)
        os.Exit(1)
    }

    // Broadcast registry
    var (
        conns     = make(map[*protocol.WSConnection]struct{})
        connsLock sync.RWMutex
        totalMsgs int64
    )

    // Debug probes
    ctrl := srv.GetControl()
    ctrl.RegisterDebugProbe("connections", func() any {
        connsLock.RLock()
        n := len(conns)
        connsLock.RUnlock()
        return n
    })
    ctrl.RegisterDebugProbe("total_messages", func() any {
        return atomic.LoadInt64(&totalMsgs)
    })

    // Reactor ticker
    go func() {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            stats := ctrl.Stats()
            fmt.Printf("[%s] Conns: %v, Msgs: %v\n",
                time.Now().Format(time.Stamp),
                stats["connections"],
                stats["total_messages"],
            )
        }
    }()

    // Broadcast handler
    handler := adapters.HandlerFunc(func(data any) error {
        buf := data.(api.Buffer)
        defer buf.Release()
        payload := buf.Bytes()
        atomic.AddInt64(&totalMsgs, 1)

        // Identify sender
        conn := api.FromContext(api.ContextFromData(data)).(*protocol.WSConnection)

        // Create one copy for broadcast
        bcast := make([]byte, len(payload))
        copy(bcast, payload)

        // Broadcast to all except sender
        connsLock.RLock()
        for ws := range conns {
            if ws != conn {
                frame := &protocol.WSFrame{
                    IsFinal:    true,
                    Opcode:     protocol.OpcodeBinary,
                    PayloadLen: int64(len(bcast)),
                    Payload:    bcast,
                }
                ws.SendFrame(frame)
            }
        }
        connsLock.RUnlock()
        return nil
    })

    // Connection tracking middleware
    track := func(next api.Handler) api.Handler {
        return adapters.HandlerFunc(func(data any) error {
            switch evt := data.(type) {
            case api.OpenEvent:
                ws := evt.Conn.(*protocol.WSConnection)
                connsLock.Lock()
                conns[ws] = struct{}{}
                connsLock.Unlock()
            case api.CloseEvent:
                ws := evt.Conn.(*protocol.WSConnection)
                connsLock.Lock()
                delete(conns, ws)
                connsLock.Unlock()
            }
            return next.Handle(data)
        })
    }
    srv.UseMiddleware(track)

    // Run server
    go func() {
        if err := srv.Run(handler); err != nil {
            fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
            os.Exit(1)
        }
    }()

    // Graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    fmt.Println("Shutting down broadcast server...")
    srv.Shutdown()
    fmt.Println("Server stopped.")
}
```


### go.mod

```go
module github.com/momentics/hioload-ws/examples/lowlevel/broadcast

go 1.23

require github.com/momentics/hioload-ws v0.0.0
```


## Running the Example

Build and run:

```bash
cd examples/lowlevel/broadcast
go build -o broadcast-server
./broadcast-server -addr=":9002" -batch=64 -ring=2048 -workers=8 -numa=0
```

Connect multiple clients:

```bash
wscat -c ws://localhost:9002
```

Any message sent from one client will be echoed to all others.

## Performance Tuning

- **Batch Size**: Increase to improve throughput at cost of latency.
- **Ring Capacity**: Must accommodate peak bursts.
- **Executor Workers**: Set equal to CPU cores per NUMA node.
- **NUMA Pinning**: Use `-numa` to bind memory and threads to optimal NUMA node.


## Debugging \& Metrics

- Register additional debug probes via `srv.GetControl().RegisterDebugProbe`.
- Expose metrics to Prometheus or custom sinks by reading `Control.Stats()`.
- Use `pprof` or `trace` for profiling reactor and executor performance.


## Graceful Shutdown

On SIGINT/SIGTERM:

1. Stop accepting new connections.
2. Stop reactor polling and executor tasks.
3. Wait for in‐flight events up to the configured shutdown timeout.
4. Cleanly close all transports and release buffers.


## License

Apache 2.0 — see [LICENSE](../../LICENSE) for details.

## Contributing

For questions, feature requests, or contributions:

- **Author**: momentics <momentics@gmail.com>  
- **Repository**: https://github.com/momentics/hioload-ws  
- **Issues & PRs**: Welcome!  

---
