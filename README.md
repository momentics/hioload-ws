# hioload-ws

High-Performance, NUMA-Aware, Zero-Copy WebSocket Library Skeleton for Go

---

**Author:** momentics <momentics@gmail.com>

---

## Overview

`hioload-ws` delivers a powerful and extensible foundation for building ultra-high-performance WebSocket servers and networking infrastructure in Go. Designed to embrace advances in modern hardware, kernel, and software best practices, this project focuses on scalable event-driven processing, memory locality, and minimizing latency under extreme loads. Its clean architecture and robust abstractions enable direct integration of core DPDK/HiPerf strategies: zero-copy, NUMA-awareness, affinity, batch-processing, lock-free data flow, as well as compatibility with both mainstream and advanced OS environments.

The repository targets backend developers, system programmers, and architects seeking a reliable starting point for custom WebSocket/real-time networking solutions demanding the lowest possible overhead and the highest levels of performance—whether as the foundation for production messaging platforms or for research into future state-of-the-art network stack patterns.

---

## Features

- **Poll-mode network IO** 
  Leverages poll-based eventing (epoll/io_uring/IOCP) for highly scalable, event-driven network concurrency. Avoids thread-per-connection models and blocks for optimal CPU resource utilization.

- **NUMA-aware memory and buffer management** 
  Architectural primitives for segmenting object and byte pools by NUMA node or CPU core, greatly improving cache locality and throughput on servers with many sockets/cores.

- **Zero-copy data paths** 
  Skeletal support for minimizing memory copying* between application and kernel or system layers, paving the way for hardware offload, DPDK and io_uring-based transport, and absolute minimization of latency.

- **Clean layered architecture** 
  Designed around interchangeable, testable abstractions: clear separations between reactor/event loops, networking, protocol framing, pooling, and external integrations.  
  Encourages best practices in dependency management, code clarity, and future extendibility.

- **Lock-free/wait-free concurrency control** 
  Optimized data path for high concurrency scenarios; avoids performance pitfalls from memory contention and unnecessary locking.

- **Batching, prefetch, and CPU affinity** 
  Base abstractions for batching IO and compute tasks, data prefetching optimizations, and robust structuring for cross-platform CPU pinning/affinity configuration.

- **Modular integration of best-in-class network technologies** 
  Easily infuses custom logic, DPDK/DPDK-like networking, true io_uring/zero-copy backends, or custom high-performance allocators with minimal changes in user-level code.

- **Mock/fake development and full testability** 
  Mock modules and test/fake implementations are first-class citizens, making it trivial to unit or integration test every interface and architectural layer in isolation.

- **Professional code comments and documentation** 
  Every source file and interface is thoroughly commented in clear English with carefully explained domain context, promoting codebase longevity, transparency, and ease of onboarding.

---

## Table of Contents

- [Project Vision](#project-vision)
- [System Requirements](#system-requirements)
- [Directory Structure](#directory-structure)
- [Detailed Layer and Directory Outline](#detailed-layer-and-directory-outline)
    - [API Layer: `/api/`](#api-layer-api)
    - [Reactor/Event Loop: `/reactor/`](#reactorevent-loop-reactor)
    - [Pooling: `/pool/`](#pooling-pool)
    - [Networking/Transport: `/transport/`](#networkingtransport-transport)
    - [WebSocket Protocol / Framing: `/protocol/`](#websocket-protocol--framing-protocol)
    - [Mocks/Fakes: `/fake/`](#mocksfakes-fake)
    - [Examples: `/examples/`](#examples-examples)
- [Zero-Copy, NUMA, and Poll Mode Support](#zero-copy-numa-and-poll-mode-support)
- [Integration & Extending for Production](#integration--extending-for-production)
- [Quick Start & Usage](#quick-start--usage)
- [Testing](#testing)
- [Best Practices and Integration Patterns](#best-practices-and-integration-patterns)
- [Limitations & Roadmap](#limitations--roadmap)
- [License & Contribution](#license--contribution)
- [Author & Credits](#author--credits)

---

## Project Vision

hioload-ws addresses the needs of organizations and communities operating ultra-high-load, latency-critical networking workloads. The project’s guiding goal is to empower Go programmers to implement robust, extreme-throughput WebSocket and binary protocol backends that exploit modern CPU architectures (multi-core, NUMA, cache lines), kernel innovations (zero-copy, io_uring, huge pages), and distributed event-processing patterns without being locked in to a specific technology or hardware vendor.

This repository is not a monolithic library or application—but a *framework skeleton* for building production or research-grade protocols. It encourages learning and experimentation with next-generation software patterns for cloud, bare-metal, and edge system deployment.

---

## System Requirements

- Go 1.21 or newer.
- Linux (kernel 6.20+) or Microsoft Windows Server 2016+.
- Minimum one modern multi-core CPU. For NUMA-aware and huge-page features, a system with multiple CPU sockets and NUMA nodes is recommended but not mandatory.
- No nonstandard Go dependencies in the codebase skeleton; new modules can be added as needed.

For full exploitation of features like io_uring, DPDK, or extensive NUMA segmentation, root/admin privileges and platform-specific setup may be required.

---

## Directory Structure

| Directory           | Description                                                             |
|---------------------|-------------------------------------------------------------------------|
| `/api/`             | Public API interfaces for all top-level abstractions                    |
| `/reactor/`         | Event-loop, poll-mode reactor logic, NUMA-awareness                     |
| `/pool/`            | Object and byte pools (NUMA/thread-aware, lock-free patterns)           |
| `/transport/`       | Network transport abstractions, interface to underlying sockets         |
| `/protocol/`        | WebSocket framing, parsing, binary/text decoding                        |
| `/fake/`            | Fully testable mock/fake implementations for unit and integration tests |
| `/examples/`        | Real-life and test applications: echo server, WSS, stress, telemetry    |
| `README.md`         | Main documentation (this file)                                          |
| `go.mod`            | Go Modules definition, cross-package dependency manager                 |

---

## Detailed Layer and Directory Outline

### API Layer: `/api/`

- Contains all public and internal **interfaces** for system primitives:
    - `Reactor`: core event-loop interface for poll-mode dispatching of connections (epoll, io_uring, kqueue, IOCP, etc).
    - `NetConn`: abstract raw or TLS socket, decoupled from Go stdlib types.
    - `BytePool`, `ObjectPool`: abstractions for zero-copy buffer management and pooled objects.
    - `NumaPoolManager`: topology-aware pool manager for NUMA/core/cpuset segregation.
    - `WebSocketConn`, `WebSocketFrame`: minimal WebSocket protocol framing and parsing contract.

- Interfaces are strictly documented in English, outlining responsibilities, expected object ownership, and guarantees.

- Isolation of contracts allows seamless patching, swapping, or extension without violating core architecture or introducing regressions.

---

### Reactor/Event Loop: `/reactor/`

- Comprises event loop and poller logic; implements glue for connection lifecycle and event handling.
- Heart of the polling (poll-mode) model: aims for single-threaded highly scalable dispatch with optional multi-reactor (one per NUMA node or CPU group).
- Links to pool, protocol, and transport via interfaces only. Platform-dependent implementations (e.g., io_uring, epoll, IOCP) can be swapped or independently developed.
- Example: NUMA/opinionated scheduling with worker pinning and affinity.
- Base stub/fake reactor included for testing; production logic is pluggable.

---

### Pooling: `/pool/`

- Lock-free and wait-free object/byte pool implementations that can be NUMA-, CPU- or thread-local.
- Ensures highest memory locality and nearly GC-free operation for short-lived, high-churn objects (frames, buffers, state structs).
- Zero-copy models use these pools to acquire/return all working buffers, avoiding memory churn and improving performance.
- Simple implementations (channel-based, `sync.Pool`-based) included for clarity; further custom or hardware-accelerated pools can be introduced without breaking API.

---

### Networking/Transport: `/transport/`

- Pure Go network connection logic abstracted over `net.Conn` with support for advanced object pool buffering.
- Prepares the ground for custom, DPDK/io_uring-based, or userspace network stacks in production.
- Defines the essential NetConn interface, with exemplary implementation wrapping the Go stdlib sockets and TLS, constructed to work with all system memory pools.
- Optimized for clean, deadlock-free use in reactor event loops and frame handlers.

---

### WebSocket Protocol / Framing: `/protocol/`

- Minimal, extensible framing logic for WebSocket as per RFC6455: frame headers, masking/unmasking, payload, message fetch, and ping/pong/close support.
- Built for zero-copy: frame struct always refers to memory buffers managed by pools; message lifetimes are well-defined; buffer ownership is explicit.
- Provides primitives to build further application-layer logic (message aggregation, fragmentation, compression, subprotocols).
- Serves as model/code base for implementing custom binary protocols with similar architectural requirements.

---

### Mocks/Fakes: `/fake/`

- Foundational stubs, fakes, and mock implementations for test-driven development and CI/CD.
- Every interface in `/api/` is covered by a minimal fake object: predictable, no-op behavior for buffer pools, reactors, transport, protocol, and more.
- Used both in standalone unit tests and as injectables for application-level testing in `/examples/`.
- Allows rapid prototyping, safe regression testing, and verification of system-wide invariants.

---

### Examples: `/examples/`

A suite of self-contained Go programs demonstrating real-world and edge-case uses of the library:

- **Echo Server**: A basic WebSocket echo demonstrating the full pipeline with pool-backed transport and protocol logic.
- **WSS (TLS) Secure Server & Client**: Showcases production-grade TLS + WebSocket integration—complete with buffer pooling and certificate validation.
- **Benchmark / Load Generation**: (planned) Multiclient simulators and stress tools to profile server performance, correctness, and tail latency.
- **Broadcast, Chat, PubSub**: (planned) Simulate clustered stateful messaging with subscriber sets.
- **Reverse Proxy**: (planned) High-performance WebSocket proxy, transparent tunneling, or multiplexing scenarios.
- **Telemetry & Diagnostics Dashboard**: (planned) Live metrics, NUMA node resource visualization, object pool statistics, and system health reporting.
- **Fault/Edge Case Testers**: (planned) Robustness, hard disconnects, protocol abuse, fuzzing web clients for error path validation.

---

## Zero-Copy, NUMA, and Poll Mode Support

hioload-ws is structured to make the most of the following advanced techniques and hardware/OS features:

### Zero-Copy Data Flows

- All payload buffers are managed by explicit pools.
- Frame structs manipulate only references—never copies—and buffer ownership is rigidly scoped (never passed to GC).
- Networking and IO APIs are designed for immediate compatibility with kernel-accelerated zero-copy (e.g., `sendfile`, `io_uring`, kernel buffer registration).

### NUMA-Awareness

- Pool managers and reactors can be instantiated per NUMA node or CPU set.
- Connections, reactors, and object pools can be pinned, minimizing remote memory access, thus reducing cross-chip data transfer bottlenecks.
- Internal stubs for NUMA and affinity logic allow straightforward integration of system utilities (`numactl`, CPU lists, explicit core mapping) in real deployments.

### Poll Mode (Polling) Eventing

- Event loops are implemented in poll-mode (no blocking per connection, no syscall per event).
- Multiple reactors per hardware NUMA socket/CPU are supported for scale-out.
- Go runtime uses poll internally for concurrency; the skeleton exposes how to extend this to build true C10M-class servers.

---

## Integration & Extending for Production

hioload-ws does not enforce any “one true stack”—developers are empowered to adapt or replace any module for:

- DPDK or netmap-based packet IO for cut-through data path and hardware offload.
- Kernel-bypass stacks for extreme throughput scenarios.
- Pinning and NUMA-aware resource management at the application or container orchestration level.
- New protocols, stateful messaging, and hybrid binary/data formats.
- Secure (TLS) in-place upgrade and certificate reloading for long-lived applications.

All source code is designed to enable this flexibility, without sacrificing code clarity or future maintainability.

---

## Quick Start & Usage

### 1. Clone the repository

```

git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws

```

### 2. Build and run tests

```

go test ./...

```
All package and integration tests (using mocks and supplied infrastructure) will run and validate project integrity.

### 3. Launch the Echo Server Example

```

go run ./examples/echo/main.go

```
- The server will listen on `:9001` and respond with an echo for each received message.
- Connect with a WebSocket client of your choice (`wscat`, `websocat`, Postman, browser JS, etc.), e.g.:
```

wscat -c ws://localhost:9001

```

### 4. Run the Secure WSS Example (if available)

```

go run ./examples/wss/server.go

```
and in a separate terminal:
```

go run ./examples/wss/client.go

```
Refer to `/examples/wss/README.md` for certificate generation steps and usage.

---

## Testing

- All interfaces, pools, and reactors have injectable mock/fake layers for unit and integration tests.
- Write new tests simply by using fakes from `/fake/`, giving full control over memory, connection, and event lifecycle.
- For extensive testing, extend `/examples/` with stress or edge-case scenarios; test both production and experimental code branches.

---

## Best Practices and Integration Patterns

hioload-ws is a starting point for industrial-grade, performance-sensitive Go networking codebases. The following practices are recommended:

- **Always acquire and release buffers via pool objects**; never allocate temporary slices in hot-data paths or protocol loops.
- **Pin worker goroutines to specific CPUs for best NUMA performance**; ensure that each reactor reads from its own set of event sources and object pools.
- **Favor batching and prefetch in network IO and protocol serialization**; minimize context switches and system call frequency.
- **Cleanly separate eventing, protocol parsing, and user logic with explicit interfaces.**
- **Write and run integration tests using fakes/mocks** before deployment with real network sockets.
- **Use platform-specific tools** (`numactl`, `taskset`, etc.) for performance investigations and affinity tuning.

---

## Limitations & Roadmap

### Why this skeleton?

hioload-ws is designed as a deeply commented and flexible reference point—not a drop-in one-size-fits-all “magic” library. Features like full WebSocket compliance, authentication, advanced binary codecs, and reliability patterns (reconnect, backpressure, dynamic scaling) are **not** implemented by default, to keep architectural surfaces clean and focused.

### Planned and Suggested Extensions

- Advanced real poll-mode event loops: pure Go io_uring, custom epoll driver, Windows IOCP integration
- DPDK, XDP, AF_XDP kernel-bypass net backend integration
- Heapless allocations: custom, hugepage-oriented allocators for frame and message buffering
- Full RFC6455 protocol compliance (including pipelining, fragmentation, masking)
- Out-of-the-box metrics and live profiling integration for performance debugging in prod
- Rich telemetry and debugging hooks for production troubleshooting
- Example-driven guides for integration with Kubernetes/Cloud deployments
- Multi-tenant messaging, sharding, or "broadcast storm" demos for Internet-scale messaging

---

## License & Contribution

hioload-ws is available under the MIT License: see `LICENSE` in the repository root for full text.

**Contributions are welcome!**  
Open issues, feature proposals, and pull requests to drive the project forward are invited. All architectural discussion, code review suggestions, and production bugfixes are encouraged—especially those accelerating state-of-the-art or enabling new deployment paradigms.

All code contributions should include clear ownership, documentation, and focus on interoperability and maintainability.  
For large refactoring or integration of new poll-mode/zero-copy tech, please open an issue first to discuss general design and roadmap fit.

---

## Author & Credits

**Developed and maintained by:**  
momentics <momentics@gmail.com>

**Special thanks:**  
The Go community, DPDK and io_uring maintainers, and the open source network programming ecosystem for ideas, tools, and feedback inspiring the architecture of this repository.

---

## Appendix: Frequently Asked Questions

### 1. How do I add my own protocol or custom logic?

Implement corresponding interfaces from `/api/`, plug your code into the event loop, and use existing pool and transport abstractions for memory and connection handling.  
Refer to `/examples/` for “bare metal” demo implementations.

### 2. Where are the high-level application features (auth, broker, clustering)?

hioload-ws intentionally avoids such features in the core skeleton to keep it maintainable and clean. These can be contributed as separate modules or companion projects built atop the provided framework.

### 3. How do I integrate with DPDK or io_uring?

Create a new module or implementation of the `Reactor`, `NetConn`, and `Pool` interfaces, leveraging hardware-accelerated libraries as needed.  
See internal comments and watch for community branch development for examples.

### 4. Is Windows supported?

Yes. Most architectural patterns (poll-mode, pool, concurrency, NUMA) are available in modern Windows OS (Server 2016+), especially for IOCP and memory affinity features.

### 5. Is code production-ready?

The skeleton code is architected for production use but intended as a template for extension and adaptation, not as a drop-in production service. Security, roadmap protocols, and robustness for production deployments are the responsibility of downstream users and maintainers.

---

For additional resources, up-to-date documentation, and advanced examples, visit the project page or open discussion threads.

---