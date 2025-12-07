// File: internal/transport/transport_windows.go
//go:build windows
// +build windows

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows-native NUMA-aware, batch-enabled transport using IOCP, WSASend/WSARecv.
// Full support for buffer pool NUMA pinning. Integrated with latest BufferPoolManager.

package transport

import (
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/windows"
)

const maxBatch = 32

type ioResult struct {
	bytes uint32
	err   error
}

type windowsTransport struct {
	recvMu       sync.Mutex
	sendMu       sync.Mutex
	socket       windows.Handle
	iocp         windows.Handle
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
	closeMu      sync.RWMutex

	readDeadline  time.Time
	writeDeadline time.Time

	// Overlapped structures must be stable in memory
	recvOverlapped windows.Overlapped
	sendOverlapped windows.Overlapped

	recvDone chan ioResult
	sendDone chan ioResult
}

// newTransportInternal creates a NUMA-aware batch transport for Windows.
func newTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	// logToFile("newTransportInternal called")
	nodeCnt := concurrency.NUMANodes()
	node := numaNode
	if node < 0 || node >= nodeCnt {
		node = 0 // Fallback: use NUMA node 0
	}

	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)
	iocp, err := windows.CreateIoCompletionPort(sock, 0, 0, 0)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("CreateIoCompletionPort: %w", err)
	}
	// Explicitly supply number of NUMA nodes to BufferPoolManager.
	bufPool := pool.NewBufferPoolManager(nodeCnt).GetPool(ioBufferSize, node)

	wt := &windowsTransport{
		socket:       sock,
		iocp:         iocp,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
		recvDone:     make(chan ioResult, 1),
		sendDone:     make(chan ioResult, 1),
	}

	// Start dispatcher
	go wt.dispatchLoop()

	return wt, nil
}

// newTransportFromConnInternal creates a NUMA-aware batch transport from an existing connection.
func newTransportFromConnInternal(conn interface{}, ioBufferSize, numaNode int) (api.Transport, error) {
	// panic("newTransportFromConnInternal called") // for debugging

	// Extract syscall.Conn
	sysConn, ok := conn.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return nil, fmt.Errorf("connection does not support SyscallConn")
	}

	rawConn, err := sysConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("SyscallConn: %w", err)
	}

	var socketHandle windows.Handle
	var sockErr error

	err = rawConn.Control(func(fd uintptr) {
		socketHandle = windows.Handle(fd)
	})
	if err != nil {
		return nil, fmt.Errorf("Control: %w", err)
	}
	if sockErr != nil {
		return nil, sockErr
	}

	// Now we have the handle. Associate with IOCP.
	nodeCnt := concurrency.NUMANodes()
	// Fallback node logic
	node := numaNode
	if node < 0 || node >= nodeCnt {
		node = 0
	}

	iocp, err := windows.CreateIoCompletionPort(socketHandle, 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateIoCompletionPort from handle: %w", err)
	}

	bufPool := pool.NewBufferPoolManager(nodeCnt).GetPool(ioBufferSize, node)

	wt := &windowsTransport{
		socket:       socketHandle,
		iocp:         iocp,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
		recvDone:     make(chan ioResult, 1),
		sendDone:     make(chan ioResult, 1),
	}

	// Start dispatcher
	go wt.dispatchLoop()

	return wt, nil
}

// newClientTransportInternal creates a new client connection on Windows using raw sockets and IOCP.
func newClientTransportInternal(addr string, ioBufferSize, numaNode int) (api.Transport, error) {
	// Resolve address using standard net package to avoid complex windows.GetAddrInfo
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr) // Force IPv4 for simplicity for now, or handle both
	if err != nil {
		return nil, fmt.Errorf("resolve addr: %w", err)
	}

	// Open Socket
	// WSA_FLAG_OVERLAPPED is required for IOCP
	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket: %w", err)
	}

	// Close socket on error if we don't return success
	// We can't defer closure blindly, only on error.

	_ = windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)

	// Connect (Blocking)
	sa := &windows.SockaddrInet4{Port: tcpAddr.Port}
	copy(sa.Addr[:], tcpAddr.IP.To4())

	err = windows.Connect(sock, sa)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("connect: %w", err)
	}

	// Associate with IOCP
	nodeCnt := concurrency.NUMANodes()
	node := numaNode
	if node < 0 || node >= nodeCnt {
		node = 0
	}

	iocp, err := windows.CreateIoCompletionPort(sock, 0, 0, 0)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("CreateIoCompletionPort: %w", err)
	}

	bufPool := pool.NewBufferPoolManager(nodeCnt).GetPool(ioBufferSize, node)

	wt := &windowsTransport{
		socket:       sock,
		iocp:         iocp,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
		recvDone:     make(chan ioResult, 1),
		sendDone:     make(chan ioResult, 1),
	}

	// Start dispatcher
	go wt.dispatchLoop()

	return wt, nil
}

func (wt *windowsTransport) dispatchLoop() {
	var bytesTransferred uint32
	var key uintptr
	var ol *windows.Overlapped
	for {
		err := windows.GetQueuedCompletionStatus(wt.iocp, &bytesTransferred, &key, &ol, windows.INFINITE)
		if ol == nil {
			if err != nil {
				// IOCP closed or heavy error?
				if err == windows.ERROR_ABANDONED_WAIT_0 {
					return // IOCP closed
				}
				// logToFile(fmt.Sprintf("Disp: IOCP Error global: %v", err))
				// Should not happen with INFINITE wait unless IOCP closed
				return
			}
			continue
		}

		res := ioResult{
			bytes: bytesTransferred,
			err:   err,
		}

		if ol == &wt.recvOverlapped {
			select {
			case wt.recvDone <- res:
			default:
				// logToFile("Disp: Recv Done channel full/abandoned")
			}
		} else if ol == &wt.sendOverlapped {
			select {
			case wt.sendDone <- res:
			default:
				// logToFile("Disp: Send Done channel full/abandoned")
			}
		} else {
			// logToFile(fmt.Sprintf("Disp: Unknown overlapped completion: %p", ol))
		}
	}
}

// Stubs for Linux transports to satisfy cross-platform compilation of transport.go on Windows
func newIoURingTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("io_uring transport not supported on Windows")
}

func newEpollTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("epoll transport not supported on Windows")
}

func newIoURingTransportFromConnInternal(conn interface{}, ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("io_uring transport not supported on Windows")
}

func newEpollTransportFromConnInternal(conn interface{}, ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("epoll transport not supported on Windows")
}

func newIoURingClientTransportInternal(addr string, ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("io_uring transport not supported on Windows")
}

func newEpollClientTransportInternal(addr string, ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("epoll transport not supported on Windows")
}

func (wt *windowsTransport) SetReadDeadline(t time.Time) error {
	wt.recvMu.Lock()
	defer wt.recvMu.Unlock()
	wt.readDeadline = t
	return nil
}

func (wt *windowsTransport) SetWriteDeadline(t time.Time) error {
	wt.sendMu.Lock()
	defer wt.sendMu.Unlock()
	wt.writeDeadline = t
	return nil
}

func (wt *windowsTransport) Recv() ([][]byte, error) {
	wt.recvMu.Lock()
	defer wt.recvMu.Unlock()

	// logToFile("Recv: Start")

	wt.closeMu.RLock()
	if wt.closed {
		wt.closeMu.RUnlock()
		return nil, api.ErrTransportClosed
	}
	wt.closeMu.RUnlock()

	wsabufs := make([]windows.WSABuf, maxBatch)
	bufs := make([][]byte, maxBatch)

	// Allocate buffers
	for i := 0; i < maxBatch; i++ {
		buf := wt.bufPool.Get(wt.ioBufferSize, wt.numaNode)
		data := buf.Bytes()
		bufs[i] = data
		wsabufs[i].Len = uint32(len(data))
		wsabufs[i].Buf = &data[0]
	}

	wt.recvOverlapped = windows.Overlapped{} // Clear it

	var received uint32
	var flags uint32

	select {
	case <-wt.recvDone:
	default:
	}

	// fmt.Println("Recv: Issuing WSARecv")
	err := windows.WSARecv(wt.socket, &wsabufs[0], uint32(maxBatch), &received, &flags, &wt.recvOverlapped, nil)
	// fmt.Printf("DEBUG: WSARecv ret: err=%v, received=%d\n", err, received)
	if err != nil && err != windows.ERROR_IO_PENDING {
		// logToFile(fmt.Sprintf("Recv: WSARecv immediate error: %v", err))
		return nil, fmt.Errorf("WSARecv batch: %w", err)
	}

	// Wait for completion
	var batchBytes uint32

	if !wt.readDeadline.IsZero() {
		dur := time.Until(wt.readDeadline)
		if dur < 0 {
			dur = 0
		}

		select {
		case res := <-wt.recvDone:
			if res.err != nil {
				// logToFile(fmt.Sprintf("Recv: Async error: %v", res.err))
				return nil, fmt.Errorf("async recv error: %w", res.err)
			}
			batchBytes = res.bytes
		case <-time.After(dur):
			// logToFile("Recv: Timeout")
			windows.CancelIoEx(wt.socket, &wt.recvOverlapped)
			<-wt.recvDone
			return nil, fmt.Errorf("read timeout")
		}
	} else {
		res := <-wt.recvDone
		if res.err != nil {
			// logToFile(fmt.Sprintf("Recv: Async error (blocking): %v", res.err))
			return nil, fmt.Errorf("async recv error: %w", res.err)
		}
		batchBytes = res.bytes
	}

	if batchBytes == 0 {
		// logToFile("Recv: EOF 0 bytes")
		return nil, io.EOF
	}

	// Calculate usage
	var consumed uint32 = 0
	resultBufs := make([][]byte, 0, maxBatch)

	for i := 0; i < maxBatch; i++ {
		if consumed >= batchBytes {
			break
		}

		capacity := wsabufs[i].Len
		remaining := batchBytes - consumed
		chunk := remaining
		if chunk > capacity {
			chunk = capacity
		}

		resultBufs = append(resultBufs, bufs[i][:chunk])
		consumed += chunk
	}

	return resultBufs, nil
}

func (wt *windowsTransport) Send(buffers [][]byte) error {
	wt.sendMu.Lock()
	defer wt.sendMu.Unlock()

	wt.closeMu.RLock()
	if wt.closed {
		wt.closeMu.RUnlock()
		return api.ErrTransportClosed
	}
	wt.closeMu.RUnlock()

	for offset := 0; offset < len(buffers); offset += maxBatch {
		end := offset + maxBatch
		if end > len(buffers) {
			end = len(buffers)
		}
		slice := buffers[offset:end]
		wsabufs := make([]windows.WSABuf, len(slice))
		for i, b := range slice {
			wsabufs[i].Len = uint32(len(b))
			wsabufs[i].Buf = &b[0]
		}

		wt.sendOverlapped = windows.Overlapped{} // Clear

		// Drain stale
		select {
		case <-wt.sendDone:
		default:
		}

		var sent uint32
		err := windows.WSASend(wt.socket, &wsabufs[0], uint32(len(wsabufs)), &sent, 0, &wt.sendOverlapped, nil)
		// fmt.Printf("DEBUG: WSASend ret: err=%v, sent=%d\n", err, sent)
		if err != nil && err != windows.ERROR_IO_PENDING {
			// logToFile(fmt.Sprintf("Send: WSASend immediate error: %v", err))
			return fmt.Errorf("WSASend batch: %w", err)
		}

		if !wt.writeDeadline.IsZero() {
			dur := time.Until(wt.writeDeadline)
			if dur < 0 {
				dur = 0
			}
			select {
			case res := <-wt.sendDone:
				if res.err != nil {
					// logToFile(fmt.Sprintf("Send: Async error: %v", res.err))
					return fmt.Errorf("async send error: %w", res.err)
				}
			case <-time.After(dur):
				// logToFile("Send: Timeout")
				windows.CancelIoEx(wt.socket, &wt.sendOverlapped)
				<-wt.sendDone
				return fmt.Errorf("write timeout")
			}
		} else {
			res := <-wt.sendDone
			if res.err != nil {
				// logToFile(fmt.Sprintf("Send: Async error (blocking): %v", res.err))
				return fmt.Errorf("async send error: %w", res.err)
			}
		}
	}
	return nil
}

func (wt *windowsTransport) Close() error {
	wt.closeMu.Lock()
	defer wt.closeMu.Unlock()

	if !wt.closed {
		wt.closed = true
		windows.CancelIoEx(wt.socket, nil)
		windows.CloseHandle(wt.iocp) // This will wake up dispatcher
		windows.Closesocket(wt.socket)
	}
	return nil
}

func (wt *windowsTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
		TLS:       false,
		OS:        []string{"windows"},
	}
}
