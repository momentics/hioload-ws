// File: internal/transport/transport.go
// Package transport
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Factory and contract for creation of NUMA-aware, zero-copy, batch transports,
// abstracting platform implementation behind unified methods.
//
// Compatible with the latest /pool and /internal/concurrency contracts.

package transport

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// TransportFactory produces blanket api.Transport instances using
// the new NUMA-aware BufferPoolManager and all required parameters.
type TransportFactory struct {
	IOBufferSize int
	NUMANode     int
}

// NewTransportFactory creates a factory for the preferred NUMA node and buffer size.
func NewTransportFactory(ioBufferSize, numaNode int) *TransportFactory {
	return &TransportFactory{
		IOBufferSize: ioBufferSize,
		NUMANode:     numaNode,
	}
}

// detectedTransportType stores the runtime-determined transport type
var detectedTransportType string
var transportTypeOnce sync.Once

func logToFile(msg string) {
	f, err := os.OpenFile("c:\\hioload-ws\\debug_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(f, "[%s] %s\n", ts, msg)
}

// detectRuntimeTransportType performs runtime detection of the best available transport
func detectRuntimeTransportType() string {
	transportTypeOnce.Do(func() {
		if runtime.GOOS == "linux" && HasIoUringSupport() {
			detectedTransportType = "io_uring"
		} else {
			detectedTransportType = "default" // epoll for linux without io_uring, or other platforms
		}
	})
	return detectedTransportType
}

// Create builds a transport using the correct platform implementation and NUMA node.
func (f *TransportFactory) Create() (api.Transport, error) {
	transportType := detectRuntimeTransportType()

	var impl api.Transport
	var err error

	switch transportType {
	case "io_uring":
		impl, err = newIoURingTransportInternal(f.IOBufferSize, f.NUMANode)
		if err != nil {
			// If io_uring fails, fall back to epoll
			impl, err = newEpollTransportInternal(f.IOBufferSize, f.NUMANode)
		}
	default:
		impl, err = newTransportInternal(f.IOBufferSize, f.NUMANode)
	}

	if err != nil {
		logToFile(fmt.Sprintf("TransportFactory: Error creating impl: %v", err))
		return nil, fmt.Errorf("transport init: %w", err)
	}
	logToFile("TransportFactory: Success")
	return &safeWrapper{impl: impl}, nil
}

// CreateFromConn builds a transport by wrapping an existing network connection.
// It extracts the underlying file descriptor/handle and upgrades it to an optimized transport.
func (f *TransportFactory) CreateFromConn(conn interface{}) (api.Transport, error) {
	// detectRuntimeTransportType() // Ensure type is detected
	// logic is similar but passes conn

	transportType := detectRuntimeTransportType()
	// fmt.Printf("TransportFactory: Upgrading conn with type='%s'\n", transportType)

	var impl api.Transport
	var err error

	switch transportType {
	case "io_uring":
		impl, err = newIoURingTransportFromConnInternal(conn, f.IOBufferSize, f.NUMANode)
		if err != nil {
			impl, err = newEpollTransportFromConnInternal(conn, f.IOBufferSize, f.NUMANode)
		}
	default:
		// Default (Windows IOCP or generic)
		impl, err = newTransportFromConnInternal(conn, f.IOBufferSize, f.NUMANode)
	}

	if err != nil {
		return nil, fmt.Errorf("transport upgrade: %w", err)
	}
	return &safeWrapper{impl: impl}, nil
}

// CreateClient establishes a new client connection using the optimized transport.
// It handles socket creation and connection establishment to ensure compatibility (e.g. exclusive IOCP on Windows).
func (f *TransportFactory) CreateClient(addr string) (api.Transport, error) {
	transportType := detectRuntimeTransportType()

	var impl api.Transport
	var err error

	switch transportType {
	case "io_uring":
		impl, err = newIoURingClientTransportInternal(addr, f.IOBufferSize, f.NUMANode)
		if err != nil {
			impl, err = newEpollClientTransportInternal(addr, f.IOBufferSize, f.NUMANode)
		}
	default:
		impl, err = newClientTransportInternal(addr, f.IOBufferSize, f.NUMANode)
	}

	if err != nil {
		return nil, fmt.Errorf("create client transport: %w", err)
	}
	return &safeWrapper{impl: impl}, nil
}

// safeWrapper synchronizes all external api.Transport calls, making transport thread-safe.
// This does not serialize I/O inside the transport but only API visibility.
type safeWrapper struct {
	impl api.Transport
	mu   sync.RWMutex
}

func (w *safeWrapper) Send(bufs [][]byte) error {
	w.mu.RLock()
	impl := w.impl
	w.mu.RUnlock()
	if impl == nil {
		return api.ErrTransportClosed
	}
	return impl.Send(bufs)
}
func (w *safeWrapper) Recv() ([][]byte, error) {
	w.mu.RLock()
	impl := w.impl
	w.mu.RUnlock()
	if impl == nil {
		return nil, api.ErrTransportClosed
	}
	return impl.Recv()
}
func (w *safeWrapper) Close() error {
	w.mu.Lock()
	impl := w.impl
	w.impl = nil
	w.mu.Unlock()
	if impl == nil {
		return nil
	}
	return impl.Close()
}
func (w *safeWrapper) Features() api.TransportFeatures {
	w.mu.RLock()
	impl := w.impl
	w.mu.RUnlock()
	if impl == nil {
		return api.TransportFeatures{}
	}
	return impl.Features()
}

func (w *safeWrapper) GetBuffer() api.Buffer {
	// wrapper implementation of GetBuffer (if supported by impl)
	// No lock needed for simple interface cast, but impl might need thread safety.
	// Actually GetBuffer on transport usually implies allocating from its internal pool.
	// We should probably lock.
	w.mu.RLock()
	impl := w.impl
	w.mu.RUnlock()

	if getter, ok := impl.(interface{ GetBuffer() api.Buffer }); ok {
		return getter.GetBuffer()
	}
	// Warning: this panic is what we are trying to fix, but at least we know where it comes from.
	// If the underlying transport doesn't support it, we can't do much without changing api.Transport interface.
	panic("safeWrapper: underlying transport does not support GetBuffer")
}
