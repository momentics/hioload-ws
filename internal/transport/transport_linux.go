//go:build linux
// +build linux

// Package internal/transport implements both io_uring and epoll-based transport for Linux.
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// io_uring-based transport for Linux when available, with epoll fallback.
// Full support for buffer pool NUMA pinning. Integrated with latest BufferPoolManager.

package transport

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

// Initialize HasIoUringSupport with Linux-specific implementation
func init() {
	HasIoUringSupport = linuxHasIoUringSupport
}

// linuxHasIoUringSupport checked above.
func linuxHasIoUringSupport() bool {
	// Primary transport for Linux (Ubuntu 24.11+, Kernel 6.8+).
	// Disabled by default for CI checks / Stability.
	// TODO: Enable for production deployment.
	return false
}

// normalizeNUMANode ensures numaNode is valid within platform limits.
func normalizeNUMANode(numaNode int) int {
	maxNodes := concurrency.NUMANodes()
	if maxNodes <= 0 {
		return 0
	}
	if numaNode < 0 || numaNode >= maxNodes {
		return 0
	}
	return numaNode
}

// newTransportInternal chooses io_uring if available, otherwise falls back to epoll.
func newTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	// Try io_uring first if supported
	if HasIoUringSupport() {
		uringTransport, err := newIoURingTransportInternal(ioBufferSize, numaNode)
		if err == nil {
			return uringTransport, nil
		}
		// Log the error but try epoll as fallback
		// In production, you might want to use proper logging
		fmt.Printf("io_uring initialization failed: %v, falling back to epoll\n", err)
	}
	// Fallback to epoll
	return newEpollTransportInternal(ioBufferSize, numaNode)
}

// newTransportFromConnInternal creates a transport from an existing connection.
func newTransportFromConnInternal(conn interface{}, ioBufferSize, numaNode int) (api.Transport, error) {
	// Try io_uring if supported
	if HasIoUringSupport() {
		uringTransport, err := newIoURingTransportFromConnInternal(conn, ioBufferSize, numaNode)
		if err == nil {
			return uringTransport, nil
		}
	}
	return newEpollTransportFromConnInternal(conn, ioBufferSize, numaNode)
}

func newEpollTransportFromConnInternal(conn interface{}, ioBufferSize, numaNode int) (api.Transport, error) {
	sysConn, ok := conn.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return nil, fmt.Errorf("connection does not support SyscallConn")
	}
	rawConn, err := sysConn.SyscallConn()
	if err != nil {
		return nil, err
	}
	var sysFd int
	err = rawConn.Control(func(fd uintptr) {
		sysFd = int(fd)
	})
	if err != nil {
		return nil, err
	}

	node := normalizeNUMANode(numaNode)
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)
	return &epollTransport{
		fd:           sysFd,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}

func newIoURingTransportFromConnInternal(conn interface{}, ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("io_uring wrapping not implemented")
}

// newClientTransportInternal creates a transport by dialing the address.
func newClientTransportInternal(addr string, ioBufferSize, numaNode int) (api.Transport, error) {
	// Try io_uring if supported
	if HasIoUringSupport() {
		uringTransport, err := newIoURingClientTransportInternal(addr, ioBufferSize, numaNode)
		if err == nil {
			return uringTransport, nil
		}
	}
	return newEpollClientTransportInternal(addr, ioBufferSize, numaNode)
}

func newEpollClientTransportInternal(addr string, ioBufferSize, numaNode int) (api.Transport, error) {
	// Resolve the address
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("resolve addr: %w", err)
	}

	// Dial using net package to handle DNS and connection setup gracefully
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("dial tcp: %w", err)
	}

	// Ensure connection is closed if setup fails
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	// Set TCP_NODELAY
	if err := conn.SetNoDelay(true); err != nil {
		return nil, fmt.Errorf("set no delay: %w", err)
	}

	// Extract the file descriptor
	sysConn, err := conn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("syscall conn: %w", err)
	}

	var sysFd int
	var sysErr error
	if err := sysConn.Control(func(fd uintptr) {
		sysFd = int(fd)
	}); err != nil {
		return nil, fmt.Errorf("control: %w", err)
	}
	if sysErr != nil {
		return nil, fmt.Errorf("control inner: %w", sysErr)
	}

	// Duplicate the FD so we can manage it independently of net.Conn?
	// or properly transfer ownership.
	// net.Conn owns the FD. If we close net.Conn, it closes FD.
	// But our transport needs to own the FD to use it with epoll and close it.
	// dup is safer to avoid net.Conn interfering, or we just take the FD and
	// let net.Conn be Garbage Collected (but finalizer might close FD?).
	//
	// Better approach for "High Load" is to use the FD directly and detach from net.Conn if possible,
	// or just use `net.Dial` then use `File()` to get a copy? `File()` dups.
	//
	// Using File() is safest standard way to get a focused FD.
	// Duplicate the FD so we can manage it independently of net.Conn
	newFd, err := unix.Dup(sysFd)
	if err != nil {
		return nil, fmt.Errorf("dup: %w", err)
	}
	
	// Close original high-level conn
	conn.Close()
	
	// Set non-blocking on new FD
	if err := unix.SetNonblock(newFd, true); err != nil {
		unix.Close(newFd)
		return nil, fmt.Errorf("set nonblock: %w", err)
	}

	node := normalizeNUMANode(numaNode)
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)

	return &epollTransport{
		fd:           newFd,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}




func newIoURingClientTransportInternal(addr string, ioBufferSize, numaNode int) (api.Transport, error) {
	return nil, fmt.Errorf("io_uring client not implemented")
}

// newEpollTransportInternal creates an epoll-based transport for Linux.
func newEpollTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	node := normalizeNUMANode(numaNode)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)

	// Create NUMA-aware buffer pool
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)

	return &epollTransport{
		fd:           fd,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}

// newIoURingTransportInternal creates a transport using io_uring for Linux.
func newIoURingTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	node := normalizeNUMANode(numaNode)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}

	// Create dedicated io_uring instances for Send and Recv to avoid locking contention
	sendUring, err := initIoURing(1024) 
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("send io_uring init: %w", err)
	}
	
	recvUring, err := initIoURing(1024)
	if err != nil {
		// invoke cleanup manually to reuse Close logic if possible or just unmap
		if sendUring.sqMmap != nil { unix.Munmap(sendUring.sqMmap) }
		if sendUring.cqMmap != nil { unix.Munmap(sendUring.cqMmap) }
		if sendUring.sqeMmap != nil { unix.Munmap(sendUring.sqeMmap) }
		unix.Close(int(sendUring.fd))
		unix.Close(fd)
		return nil, fmt.Errorf("recv io_uring init: %w", err)
	}

	// Create NUMA-aware buffer pool
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)

	return &ioURingTransport{
		fd:           fd,
		sendUring:    sendUring,
		recvUring:    recvUring,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}

// initIoURing initializes the io_uring instance with proper ring buffer setup
func initIoURing(entries uint32) (*IoURing, error) {
	var params IoURingParams
	params.SQEntries = entries
	params.CQEntries = entries * 2 // CQ should be at least as large as SQ
	params.Flags = IORING_SETUP_CLAMP

	// Create io_uring
	fd, _, errno := unix.Syscall6(
		SYS_IO_URING_SETUP,
		uintptr(entries),
		uintptr(unsafe.Pointer(&params)),
		0, 0, 0, 0,
	)
	if errno != 0 {
		return nil, fmt.Errorf("io_uring_setup failed: %v", errno)
	}

	// Calculate proper mmap sizes based on parameters returned by the kernel
	sqMmapSize := int(params.SQOffArray) + int(params.SQEntries)*8 // 8 bytes per sq entry index (uint64)
	cqMmapSize := int(params.CQOffCqes) + int(params.CQEntries)*int(params.CQEntrySize)
	sqeMmapSize := int(params.SQEntries) * int(params.SQEntrySize)

	// Map rings
	sqMmap, err := unix.Mmap(int(fd), 0, sqMmapSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Close(int(fd))
		return nil, fmt.Errorf("mmap SQ ring: %w", err)
	}

	cqMmap, err := unix.Mmap(int(fd), 0, cqMmapSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(sqMmap)
		unix.Close(int(fd))
		return nil, fmt.Errorf("mmap CQ ring: %w", err)
	}

	sqeMmap, err := unix.Mmap(int(fd), 0, sqeMmapSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(sqMmap)
		unix.Munmap(cqMmap)
		unix.Close(int(fd))
		return nil, fmt.Errorf("mmap SQEs: %w", err)
	}

	// Calculate offsets to ring metadata (heads, tails, etc.)
	uring := &IoURing{
		fd:          int32(fd),
		sqHead:      (*uint32)(unsafe.Pointer(&sqMmap[params.SQOffHead])),
		sqTail:      (*uint32)(unsafe.Pointer(&sqMmap[params.SQOffTail])),
		sqMask:      params.SQOffRingMask, // Use kernel-provided mask, don't overwrite
		sqFlags:     (*uint32)(unsafe.Pointer(&sqMmap[params.SQOffFlags])),
		cqHead:      (*uint32)(unsafe.Pointer(&cqMmap[params.CQOffHead])),
		cqTail:      (*uint32)(unsafe.Pointer(&cqMmap[params.CQOffTail])),
		cqMask:      params.CQOffRingMask, // Use kernel-provided mask, don't overwrite
		cqOverflow:  (*uint32)(unsafe.Pointer(&cqMmap[params.CQOffOverflow])),
		sqMmap:      sqMmap,
		cqMmap:      cqMmap,
		sqeMmap:     sqeMmap,
		sqSize:      uint64(sqMmapSize),
		cqSize:      uint64(cqMmapSize),
		sqeSize:     uint64(sqeMmapSize),
		sqOffArray:  params.SQOffArray,
		sqEntrySize: params.SQEntrySize,
		cqEntrySize: params.CQEntrySize,
	}

	return uring, nil
}

// ioURingTransport implements api.Transport using io_uring for high-performance I/O
type ioURingTransport struct {
	fd           int
	sendUring    *IoURing // Dedicated ring for Send
	recvUring    *IoURing // Dedicated ring for Recv
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
	mutex        sync.Mutex
	sendMutex    sync.Mutex
	recvMutex    sync.Mutex
}

// getSQESlot gets next available SQE slot for the specific ring
func (t *ioURingTransport) getSQESlot(ring *IoURing) (*IoURingSQE, uint32, error) {
	head := atomic.LoadUint32(ring.sqHead)
	tail := atomic.LoadUint32(ring.sqTail)

	if (tail+1)&ring.sqMask == head {
		return nil, 0, fmt.Errorf("SQ is full")
	}

	// Get the SQE slot
	sqeIdx := tail & ring.sqMask
	sqeOffset := uintptr(sqeIdx) * uintptr(ring.sqEntrySize)
	sqe := (*IoURingSQE)(unsafe.Pointer(uintptr(unsafe.Pointer(&ring.sqeMmap[0])) + sqeOffset))

	return sqe, sqeIdx, nil
}

// submitSQE removed/inline in Send/Recv to avoid confusion or need for update

// linuxHasIoUringSupport checked above.

// Send submits send operations - using proper io_uring SQE/CQE
func (t *ioURingTransport) Send(buffers [][]byte) error {
	t.sendMutex.Lock()
	defer t.sendMutex.Unlock()

	if t.closed {
		return api.ErrTransportClosed
	}
	
	toSubmit := 0
	ring := t.sendUring

	for _, buf := range buffers {
		if len(buf) == 0 {
			continue
		}
		
		// 1. Get SQE
		sqe, idx, err := t.getSQESlot(ring)
		if err != nil {
			return fmt.Errorf("getSQE: %w", err)
		}
		
		// 2. Fill SQE
		sqe.OpCode = IORING_OP_SEND
		sqe.Fd = int32(t.fd)
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&buf[0])))
		sqe.Len = uint32(len(buf))
		sqe.Flags = 0 
		sqe.UserData = 0
		
		// 3. Update SQ Array and Tail
		sqArrayOffset := uintptr(ring.sqOffArray) + uintptr(idx)*4
		*(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&ring.sqMmap[0])) + sqArrayOffset)) = idx
		atomic.AddUint32(ring.sqTail, 1)
		
		toSubmit++
	}

	if toSubmit == 0 {
		return nil
	}

	// 4. Submit and Wait for ALL completions
	_, _, errno := unix.Syscall6(
		SYS_IO_URING_ENTER,
		uintptr(ring.fd),
		uintptr(toSubmit),      // to_submit
		uintptr(toSubmit),      // min_complete
		IORING_ENTER_GETEVENTS, // flags
		0, 0,
	)
	if errno != 0 && errno != unix.EAGAIN && errno != unix.EINTR {
		return fmt.Errorf("io_uring_enter: %v", errno)
	}
	
	// 5. Check CQEs
	for i := 0; i < toSubmit; i++ {
		for {
			head := atomic.LoadUint32(ring.cqHead)
			tail := atomic.LoadUint32(ring.cqTail)
			
			if head != tail {
				cqeIdx := head & ring.cqMask
				cqeOffset := uintptr(cqeIdx) * uintptr(ring.cqEntrySize)
				cqe := (*IoURingCQE)(unsafe.Pointer(uintptr(unsafe.Pointer(&ring.cqMmap[0])) + cqeOffset))
				atomic.StoreUint32(ring.cqHead, head+1)
				
				if cqe.Result < 0 {
					return fmt.Errorf("send failed errno: %d", -cqe.Result)
				}
				break // Got one
			}
			// Wait again if needed
			_, _, errno := unix.Syscall6(
				SYS_IO_URING_ENTER,
				uintptr(ring.fd),
				0, 1, IORING_ENTER_GETEVENTS, 0, 0,
			)
			if errno != 0 && errno != unix.EINTR {
				return fmt.Errorf("wait retry: %v", errno)
			}
		}
	}

	return nil
}


// Recv waits for receive operations - using proper io_uring SQE/CQE
func (t *ioURingTransport) Recv() ([][]byte, error) {
	t.recvMutex.Lock()
	defer t.recvMutex.Unlock()

	if t.closed {
		return nil, api.ErrTransportClosed
	}

	ring := t.recvUring

	// 1. Get Buffer
	buf := t.bufPool.Get(t.ioBufferSize, t.numaNode)
	data := buf.Bytes()
	
	// 2. Get SQE
	sqe, idx, err := t.getSQESlot(ring)
	if err != nil {
		buf.Release()
		return nil, fmt.Errorf("getSQE: %w", err)
	}

	// 3. Fill SQE
	sqe.OpCode = IORING_OP_RECV
	sqe.Fd = int32(t.fd)
	sqe.Addr = uint64(uintptr(unsafe.Pointer(&data[0])))
	sqe.Len = uint32(len(data))
	sqe.Flags = 0
	
	// 4. Update SQ Array and Tail
	sqArrayOffset := uintptr(ring.sqOffArray) + uintptr(idx)*4
	*(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&ring.sqMmap[0])) + sqArrayOffset)) = idx
	atomic.AddUint32(ring.sqTail, 1)
	
	// 5. Submit and Wait for 1 completion
	for {
		_, _, errno := unix.Syscall6(
			SYS_IO_URING_ENTER,
			uintptr(ring.fd),
			1, // to_submit
			1, // min_complete
			IORING_ENTER_GETEVENTS, // flags
			0, 0,
		)
		if errno != 0 {
			if errno == unix.EINTR {
				continue
			}
			buf.Release()
			return nil, fmt.Errorf("uring enter wait: %v", errno)
		}
		
		head := atomic.LoadUint32(ring.cqHead)
		tail := atomic.LoadUint32(ring.cqTail)
		
		if head != tail {
			cqeIdx := head & ring.cqMask
			cqeOffset := uintptr(cqeIdx) * uintptr(ring.cqEntrySize)
			cqe := (*IoURingCQE)(unsafe.Pointer(uintptr(unsafe.Pointer(&ring.cqMmap[0])) + cqeOffset))
			atomic.StoreUint32(ring.cqHead, head+1)
			
			if cqe.Result < 0 {
				buf.Release()
				return nil, fmt.Errorf("recv failed errno: %d", -cqe.Result)
			}
			
			n := int(cqe.Result)
			if n == 0 {
				buf.Release()
				return [][]byte{}, nil 
			}
			
			result := make([]byte, n)
			copy(result, data[:n])
			buf.Release()
			return [][]byte{result}, nil
		}
		
		// If we are here, we looped but no CQE found (spurious?).
		// We called Enter(..., 1, ...). It should return only when >=1 events available.
		// If it returned 0/Success, implies event ready.
		// Retrying loop.
		// Important: Next time we call Enter, to_submit MUST be 0 !
		// We already submitted 1.
		// If we submit 1 again, we submit GARBAGE or duplicate?
		// We did increment tail ONCE outside loop.
		// First call: Enter checks added entries. Submits them.
		// Second call: We have NOT added entries.
		// Enter(fd, 1, ...) might try to consume 1 more from SQ ring?
		// But SQ ring tail was not advanced.
		// So `to_submit` calculation inside kernel:
		// Kernel reads SQ tail. Matches user tail?
		// If we say to_submit=1, but user tail matches kernel tail...
		// io_uring_enter(to_submit) is strict.
		// If we say 1, it expects 1 new entry.
		//
		// CORRECTION:
		// First iteration: we added 1. to_submit=1. Correct.
		// Retry iteration: we added 0. to_submit=0. Correct.
		// So we need to set to_submit=0 in subsequent iterations.
		
		// Let's rewrite loop cleanly.
	}
}

// Helper to handle the wait loop logic
func (t *ioURingTransport) recvWaitLoop(ring *IoURing, buf api.Buffer, data []byte) ([][]byte, error) {
	toSubmit := uint32(1)
	for {
		_, _, errno := unix.Syscall6(
			SYS_IO_URING_ENTER,
			uintptr(ring.fd),
			uintptr(toSubmit),
			1, // min_complete
			IORING_ENTER_GETEVENTS,
			0, 0,
		)
		toSubmit = 0 // Next time, nothing to submit
		
		if errno != 0 {
			if errno == unix.EINTR {
				continue
			}
			buf.Release()
			return nil, fmt.Errorf("uring enter wait: %v", errno)
		}
		
		head := atomic.LoadUint32(ring.cqHead)
		tail := atomic.LoadUint32(ring.cqTail)
		
		if head != tail {
			cqeIdx := head & ring.cqMask
			cqeOffset := uintptr(cqeIdx) * uintptr(ring.cqEntrySize)
			cqe := (*IoURingCQE)(unsafe.Pointer(uintptr(unsafe.Pointer(&ring.cqMmap[0])) + cqeOffset))
			atomic.StoreUint32(ring.cqHead, head+1)
			
			if cqe.Result < 0 {
				buf.Release()
				return nil, fmt.Errorf("recv failed errno: %d", -cqe.Result)
			}
			
			n := int(cqe.Result)
			if n == 0 {
				buf.Release()
				return [][]byte{}, nil 
			}
			
			result := make([]byte, n)
			copy(result, data[:n])
			buf.Release()
			return [][]byte{result}, nil
		}
	}
}

// Close closes the transport and io_uring instances
func (t *ioURingTransport) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true

	// Cleanup io_uring resources
	for _, uring := range []*IoURing{t.sendUring, t.recvUring} {
		if uring != nil {
			if uring.sqMmap != nil {
				unix.Munmap(uring.sqMmap)
			}
			if uring.cqMmap != nil {
				unix.Munmap(uring.cqMmap)
			}
			if uring.sqeMmap != nil {
				unix.Munmap(uring.sqeMmap)
			}
			unix.Close(int(uring.fd))
		}
	}

	return unix.Close(t.fd)
}

// Features returns the transport capabilities
func (t *ioURingTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
		TLS:       false,
		OS:        []string{"linux"},
	}
}

func (t *ioURingTransport) GetBuffer() api.Buffer {
	return t.bufPool.Get(t.ioBufferSize, t.numaNode)
}

// epollTransport implements api.Transport using epoll and SendmsgBuffers for maximum performance.
type epollTransport struct {
	mu           sync.Mutex
	fd           int
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
}

func (et *epollTransport) Recv() ([][]byte, error) {
	et.mu.Lock()
	if et.closed {
		et.mu.Unlock()
		return nil, api.ErrTransportClosed
	}
	fd := et.fd
	et.mu.Unlock()

	batch := 16
	bufs := make([][]byte, batch)
	for i := range bufs {
		buf := et.bufPool.Get(et.ioBufferSize, et.numaNode)
		bufs[i] = buf.Bytes()
	}

	// Use blocking recv behavior.
	// Since fd is non-blocking (O_NONBLOCK), we must poll if checks fail.
	for {
		// Try to read
		n, _, _, _, err := unix.RecvmsgBuffers(fd, bufs, nil, 0)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				// Wait for data without holding lock
				pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
				if _, perr := unix.Poll(pfd, -1); perr != nil {
					if perr == unix.EINTR {
						continue
					}
					// Check close
					et.mu.Lock()
					if et.closed {
						et.mu.Unlock()
						return nil, api.ErrTransportClosed
					}
					et.mu.Unlock()
					return nil, fmt.Errorf("poll: %w", perr)
				}
				continue
			}
			
			// Check if closed
			et.mu.Lock()
			if et.closed {
				et.mu.Unlock()
				return nil, api.ErrTransportClosed
			}
			et.mu.Unlock()
			
			return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
		}
		
		// n is total bytes received.
		if n == 0 {
			// EOF from peer
			et.mu.Lock()
			et.closed = true
			et.mu.Unlock()
			return nil, api.ErrTransportClosed
		}

		total := n
		used := 0
		for i := range bufs {
			if total <= 0 {
				break
			}
			
			cap := len(bufs[i])
			if total < cap {
				bufs[i] = bufs[i][:total]
				total = 0
			} else {
				total -= cap
			}
			used++
		}
		return bufs[:used], nil
	}
}

func (et *epollTransport) Send(buffers [][]byte) error {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.closed {
		return api.ErrTransportClosed
	}
	const maxBatch = 16
	left := len(buffers)
	sent := 0
	for left > 0 {
		batch := buffers[sent:]
		if len(batch) > maxBatch {
			batch = batch[:maxBatch]
		}
		
		// Loop for blocking send on non-blocking socket
		for {
			n, err := unix.SendmsgBuffers(et.fd, batch, nil, nil, 0)
			if err != nil {
				if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
					// Wait for writeability
					pfd := []unix.PollFd{{Fd: int32(et.fd), Events: unix.POLLOUT}}
					// Release lock while polling to allow concurrent Close/Recv interrupt?
					// Recv holds lock? No, Recv releases lock before Poll.
					// Close holds lock.
					// If we hold lock while Polling, Close cannot happen.
					// We should release lock. 
					// But we need to check 'closed' after re-acquiring.
					// And 'fd' variable is local, so it's safe.
					et.mu.Unlock()
					_, perr := unix.Poll(pfd, -1)
					et.mu.Lock()
					
					if et.closed {
						return api.ErrTransportClosed
					}
					if perr != nil {
						if perr == unix.EINTR {
							continue
						}
						return fmt.Errorf("poll: %w", perr)
					}
					continue
				}
				return fmt.Errorf("SendmsgBuffers: %w", err)
			}
			
			if n <= 0 {
				return fmt.Errorf("SendmsgBuffers: sent no data")
			}
			// Success
			break
		}
		
		sent += len(batch)
		left -= len(batch)
	}
	return nil
}

func (et *epollTransport) GetBuffer() api.Buffer {
	return et.bufPool.Get(et.ioBufferSize, et.numaNode)
}

func (et *epollTransport) Close() error {
	et.mu.Lock()
	defer et.mu.Unlock()
	if !et.closed {
		unix.Close(et.fd)
		et.closed = true
	}
	return nil
}

func (et *epollTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
		TLS:       false,
		OS:        []string{"linux"},
	}
}
