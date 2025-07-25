// File: examples/echo/README.md
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Echo WebSocket Server Example (updated for hioload-ws /server facade)
//
// ## Overview
//
// This example demonstrates how to build a zero-copy, high-performance WebSocket echo server
// using the new `server` facade of the `hioload-ws` library. It accepts WebSocket connections,
// performs the RFC6455 handshake, and echoes every binary frame back to the sender with minimal overhead.
//
// ## Build & Run
//
//   go run examples/echo/main.go -addr=":9001"
//
//
// ## Command-Line Flags
//
//   -addr : WebSocket listen address (default ":9001")
//
// ## How It Works
//
// 1. **Configuration**: Uses `server.DefaultConfigServer()` to obtain sane defaults (IO buffer size,
//    channel size, NUMA node).
// 2. **Facade Initialization**: Calls `server.NewServer(cfg)` to create a `*server.Server`.
// 3. **Listener Creation**: Uses `srv.Accept()` to blockingly accept and handshake on new TCP connections.
// 4. **Connection Handling**: For each `*protocol.WSConnection`, spawns a goroutine that
//    reads incoming frames via `RecvZeroCopy()` and echoes them back via `SendFrame()`.
// 5. **Zero-Copy & NUMA**: All payload buffers originate from the NUMA-aware buffer pool,
//    avoiding extra allocations and copies.
// 6. **Graceful Shutdown**: Listens for SIGINT/SIGTERM, then closes `listener` and `srv`, exiting cleanly.
//
// ## Key Concepts
//
// - **Zero-Copy**: Payloads are delivered as pooled `api.Buffer` slices, avoiding heap allocations.
// - **Batch-Mode**: `RecvZeroCopy` may deliver multiple buffers per call.
// - **Lock-Free**: Handshake and transport primitives use lock-free I/O paths under the hood.
// - **NUMA-Aware**: Buffers are allocated from the configured NUMA node pool.
// - **Cross-Platform**: Works on Linux (epoll) and Windows (IOCP).
//
// For further details, refer to the hioload-ws `server` and `protocol` package documentation.
