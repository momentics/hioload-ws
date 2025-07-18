# hioload-ws

High-Performance, NUMA-Aware, Zero-Copy WebSocket Library Skeleton for Go 
**Author:** momentics <momentics@gmail.com>

## Features

- Poll-mode network IO (epoll/io_uring/IOCP-ready)
- NUMA-aware object/byte pool management
- Zero-copy data flow design (ZC-capable)
- Clean layered architecture for extensibility
- Lock-free/wait-free structures for high concurrency
- Support for batching, prefetch, and CPU affinity
- Modular: add real DPDK/io_uring logic with minimal refactoring
- Full test coverage using mock/fake modules
- Fully commented, professional Go code

## Directory Structure

- `/api/`     — Public interfaces (Reactor, Pool, WSConn)
- `/reactor/` — Reactor/event loop logic, NUMA integration
- `/pool/`    — Object and byte pools (thread/NUMA aware)
- `/transport/` — Transport layer with pool-backed buffers
- `/protocol/` — WebSocket frame/protocol parsing
- `/fake/`    — Fake/mock objects for development
- `/examples/` — Example server and usage

## Quick Start

1. Clone the repository.
2. Run test examples:
   `go test ./...`
3. Run example WebSocket echo server: 
   `go run ./examples/echo/main.go`

## License

MIT License (see LICENSE for details)

## Author

momentics <momentics@gmail.com>
