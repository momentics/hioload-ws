# Broadcast / Chat Server Example (hioload-ws, Without HTTP)

## Overview

This example demonstrates a native, zero-copy WebSocket **broadcast server** using the `hioload-ws` library without involving any HTTP logic. All connected clients receive any message broadcasted by others in real time.

It showcases:
- Fully event-driven, batched poll-mode connection handling
- NUMA-aware memory pooling and zero-copy buffer usage
- Thread-safe registry of all active clients
- Explicit disconnect handling via events
- No dependencies on `net/http` — native transport only
- Debug probes and live metrics using the `Control` API

---

## Functional Summary

- Clients connect via native WebSocket acceptor (TCP-based)
- Each message received from a client is broadcasted to all others
- Messages are delivered using zero-copy buffered memory
- Debug metrics are exposed with probe `"active_clients"`

---

## Features Demonstrated

| Feature                  | Description                                                                 |
|-------------------------|-----------------------------------------------------------------------------|
| Zero-copy Broadcast      | Messages are never re-allocated in user code; buffers reused across clients |
| NUMA-aware Buffer Pool   | Buffers are allocated from the nearest NUMA node's pool for locality         |
| Poll-Mode Reactor        | No goroutine per connection: a shared event loop handles all frames          |
| Registry                 | Active clients stored in sync.RWMutex map → used for broadcast distribution  |
| Middleware Handler Chain | Logging, panic-recovery, and metrics wrappers                               |
| Debug Probe              | Exports "active_clients" count via `Control.Stats()`                         |

---

## Important Concepts

### WebSocketConnection
Core session abstraction wrapping framed transport and message parsing for each client.

### BufferPool
A NUMA-aware memory allocator that provides pre-allocated byte slices used for I/O.

### BroadcastRegistry
Custom handler that maps each `WSConnection` to a unique session, enabling targeted or full broadcasts.

### WSFrame
Low-level protocol-framed representation of message: opcode + payload + flags. Used with `conn.Send()`.

### NUMA Node and Affinity
Modern CPUs are NUMA-based: memory closest to a CPU core is faster to access if threads and buffers are co-located. This example respects that via thread pinning and pool alignment.

---

## How to Build & Run

1. Ensure Go 1.23+ is installed
2. Clone repo and navigate into broadcast directory:
```

cd examples/broadcast
go build -o broadcast-server

```

3. Run the server:
```

./broadcast-server -addr=":9002" -shards=32 -workers=8 -batch=32 -ring=2048

```

---

## Flags and Parameters

| Flag         | Description                                                                                   |
|--------------|-----------------------------------------------------------------------------------------------|
| `-addr`      | TCP bind address (e.g., `:9002`)                                                              |
| `-shards`    | Number of session shards (default 16). Used for sharded lock-free session context registry    |
| `-workers`   | Number of executor goroutines. Should match number of physical CPU cores or NUMA balance      |
| `-batch`     | Number of events processed by the poller in each cycle                                        |
| `-ring`      | Capacity of the internal ring buffer used between poller and handlers                         |
| `-numa`      | Optional NUMA node to prefer for memory operations (-1 = auto)                                |

---

## Test With wscat

1. Open two different terminals:

```

wscat -c ws://localhost:9002

```

2. Send a message like:
```

Hello everyone!

```

3. All clients receive this message simultaneously.

---

## Debug Probes

Example probe available:

```

GetControl().RegisterDebugProbe("active_clients", func() any {
return registry.Size()
})

```

View output via logs or your metric exporter.

---

## Graceful Shutdown

SIGINT or SIGTERM triggers:
- Listener `.Close()`
- Session cleanup
- Worker pool shutdown
- Metrics flushed

---

## Notes

- No use of HTTP or `net/http`
- Ready for production: minimal memory copy, bounded resource usage
- Fully portable across Linux and Windows (epoll/IOCP)

---
