# Reactor Echo Example (Cross-Platform)

Author: momentics <momentics@gmail.com>

---

## Overview

This example demonstrates a cross-platform echo server built on top of the `reactor` module of the `hioload-ws` project. It uses low-level poll-mode networking primitives (`epoll` on Linux, `IOCP` on Windows) to efficiently manage TCP socket connections and echo back received data in real-time.

This is a minimal but extensible example, serving as a test harness and practical verification of the core functionality of the reactor subsystem.

---

## Features

- Fully asynchronous event-driven architecture
- Cross-platform support (Linux, Windows)
- Uses native pollers:
  - Linux: `epoll`
  - Windows: `IOCP`
- Zero external dependencies — uses only the Go standard library
- Clear console logging for debugging and verification
- Platform-specific implementation of raw socket I/O
- Clean reactor abstraction layer via interface
- Demonstrates safe concurrent event-driven I/O dispatching

---

## Components

### 1. `main.go`

- Entry point of the echo server
- Accepts incoming TCP connections on port `:9002`
- Extracts raw file descriptor or handle (`fd`) from each `net.TCPConn`
- Registers each `fd` with the cross-platform `reactor` system
- Handles read/write events via callback (`FDCallback`)
- Uses platform-specific read/write functions (`readFromSocket`, `writeToSocket`, `closeSocket`)

### 2. `socket_unix.go`

- Used only on Unix-based systems (Linux/macOS)
- Provides `readFromSocket`, `writeToSocket`, and `closeSocket` using `syscall` package
- Uses classic POSIX-style `Read`, `Write`, and `Close`

### 3. `socket_windows.go`

- Used only on Windows systems
- Wraps system calls for raw `syscall.Read`, `syscall.Write`, and `syscall.Closesocket`
- Microsoft Windows–safe implementation using Go syscalls

---

## How It Works

1. Server listens on TCP port `:9002`
2. On connection, retrieves OS-level socket descriptor via `SyscallConn`
3. Registers descriptor or handle with the reactor using `Register(fd, EventRead, callback)`
4. Reactor internally uses:
   - `epoll_wait` loop (Linux)
   - `GetQueuedCompletionStatus` loop (Windows)
5. On readable event, `readFromSocket` is called to receive bytes
6. The bytes are logged and passed to `writeToSocket` to echo back
7. Closed connections are detected on read failure or zero-byte input
8. Resources are safely closed using `closeSocket`

---

## Running the Example

Make sure you are in the root of the project directory.

**On Linux/macOS**

```

go run ./examples/reactor_echo/

```

**On Windows (PowerShell or CMD)**

```

go run .\examples\reactor_echo\

```

> Do NOT run `go run main.go` — this bypasses platform-specific files and fails.

### Console Output

Example output on connection:

```

[reactor-echo] Listening on :9002 ...
[reactor-echo] Reactor initialized
[reactor-echo] Accepted connection from 127.0.0.1:57432
[reactor-echo] Received (13 bytes): Hello World!
[reactor-echo] Sent (13 bytes) back to client
[reactor-echo] Connection closed by peer (fd=12)

```

---

## Testing the Echo Server

You can use any WebSocket test tool like [wscat], [websocat], `telnet`, or create a custom Go/JS/Python based TCP client.

### Example with netcat (Linux):

```

nc 127.0.0.1 9002

```

### Example with PowerShell (Windows 10+):

```

\$client = New-Object System.Net.Sockets.TcpClient('localhost', 9002)
\$stream = \$client.GetStream()
\$writer = New-Object IO.StreamWriter \$stream
\$reader = New-Object IO.StreamReader \$stream
\$writer.WriteLine("hello from PowerShell!")
\$writer.Flush()
\$response = \$reader.ReadLine()
\$response

```

---

## Internal Design Notes

- `reactor.Reactor` is the common interface implemented differently based on OS
- Uses `sync.Map` to store registered callbacks per file descriptor
- Uses Go's `runtime.LockOSThread()` on Linux (optional for affinities)
- Avoids using goroutine-per-connection model
- Demonstrates lock-free networking principles (poll-loop model)
- Uses safe `recover()`-guarded callback executions
- Shows clean lifecycle: register -> poll -> handle -> unregister/close

---

## File Descriptions

| File                  | Description                                |
|-----------------------|--------------------------------------------|
| `main.go`             | Main logic for the TCP reactor echo server |
| `socket_unix.go`      | BSD/Linux socket syscall wrappers          |
| `socket_windows.go`   | Windows socket syscall wrappers            |
| `README.md`           | English-language documentation             |
| `README_RU.md`        | Russian-language documentation             |

---

## Requirements

- Go ≥ 1.21
- No cgo dependencies
- Windows 10/11, Server 2016+, or Linux 6.0+
- IPv4 loopback access

---

## Licensing

MIT License © 2025 — momentics <momentics@gmail.com>
