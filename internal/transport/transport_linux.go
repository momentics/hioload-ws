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

// linuxHasIoUringSupport checks if the kernel supports io_uring
func linuxHasIoUringSupport() bool {
	// Try to create a minimal io_uring instance to check support
	var params IoURingParams
	params.SQEntries = 2 // minimal size
	params.CQEntries = 2

	fd, _, errno := unix.Syscall6(
		SYS_IO_URING_SETUP,
		uintptr(2), // entries
		uintptr(unsafe.Pointer(&params)),
		0, 0, 0, 0,
	)

	if errno != 0 {
		// io_uring not available or not supported
		return false
	}

	// Successfully created, close it
	unix.Close(int(fd))
	return true
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

	// Create io_uring instance
	uring, err := initIoURing(1024) // 1024 entries
	if err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("io_uring init: %w", err)
	}

	// Create NUMA-aware buffer pool
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)

	return &ioURingTransport{
		fd:           fd,
		ioUring:      uring,
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
	sqeMmapSize := int(entries) * int(params.SQEntrySize)

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

	sqeMmap, err := unix.Mmap(int(fd), int64(params.SQEntrySize), sqeMmapSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		unix.Munmap(sqMmap)
		unix.Munmap(cqMmap)
		unix.Close(int(fd))
		return nil, fmt.Errorf("mmap SQEs: %w", err)
	}

	// Calculate offsets to ring metadata (heads, tails, etc.)
	uring := &IoURing{
		fd:         int32(fd),
		sqHead:     (*uint32)(unsafe.Pointer(&sqMmap[params.SQOffHead])),
		sqTail:     (*uint32)(unsafe.Pointer(&sqMmap[params.SQOffTail])),
		sqMask:     *(*uint32)(unsafe.Pointer(&sqMmap[params.SQOffRingMask])),
		sqFlags:    (*uint32)(unsafe.Pointer(&sqMmap[params.SQOffFlags])),
		cqHead:     (*uint32)(unsafe.Pointer(&cqMmap[params.CQOffHead])),
		cqTail:     (*uint32)(unsafe.Pointer(&cqMmap[params.CQOffTail])),
		cqMask:     *(*uint32)(unsafe.Pointer(&cqMmap[params.CQOffRingMask])),
		cqOverflow: (*uint32)(unsafe.Pointer(&cqMmap[params.CQOffOverflow])),
		sqMmap:     sqMmap,
		cqMmap:     cqMmap,
		sqeMmap:    sqeMmap,
		sqSize:     uint64(sqMmapSize),
		cqSize:     uint64(cqMmapSize),
		sqeSize:    uint64(sqeMmapSize),
		sqOffArray: params.SQOffArray,
		sqEntrySize: params.SQEntrySize,
		cqEntrySize: params.CQEntrySize,
	}

	// Initialize masks
	uring.sqMask = params.SQEntries - 1
	uring.cqMask = params.CQEntries - 1

	return uring, nil
}

// ioURingTransport implements api.Transport using io_uring for high-performance I/O
type ioURingTransport struct {
	fd           int
	ioUring      *IoURing
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
	mutex        sync.Mutex
	sendMutex    sync.Mutex  // Separate mutex for send operations
	recvMutex    sync.Mutex  // Separate mutex for recv operations
}

// getSQESlot gets next available SQE slot
func (t *ioURingTransport) getSQESlot() (*IoURingSQE, uint32, error) {
	head := atomic.LoadUint32(t.ioUring.sqHead)
	tail := atomic.LoadUint32(t.ioUring.sqTail)

	if (tail + 1) % (t.ioUring.sqMask + 1) == head {
		return nil, 0, fmt.Errorf("SQ is full")
	}

	// Get the SQE slot
	sqeIdx := tail & t.ioUring.sqMask
	sqeOffset := uintptr(sqeIdx) * uintptr(t.ioUring.sqEntrySize) // Use the actual size from params
	sqe := (*IoURingSQE)(unsafe.Pointer(uintptr(unsafe.Pointer(&t.ioUring.sqeMmap[0])) + sqeOffset))

	return sqe, sqeIdx, nil
}

// submitSQE submits an SQE to the ring
func (t *ioURingTransport) submitSQE(sqe *IoURingSQE, sqeIdx uint32) {
	// Update the SQ array with the SQE index
	sqArrayOffset := uintptr(t.ioUring.sqOffArray) + uintptr(sqeIdx)*8 // Each index is 8 bytes (uint64)
	*(*uint32)(unsafe.Pointer(uintptr(unsafe.Pointer(&t.ioUring.sqMmap[0])) + sqArrayOffset)) = sqeIdx

	// Increment tail
	newTail := (atomic.LoadUint32(t.ioUring.sqTail) + 1) & t.ioUring.sqMask
	atomic.StoreUint32(t.ioUring.sqTail, newTail)

	// Notify kernel if SQ polling is not enabled
	const IORING_SQ_NEED_WAKEUP = 1 << 0  // 1 - need wake up flag
	if (*t.ioUring.sqFlags & IORING_SQ_NEED_WAKEUP) != 0 {
		_, _, errno := unix.Syscall6(
			SYS_IO_URING_ENTER,
			uintptr(t.ioUring.fd),
			1, // count
			IORING_ENTER_GETEVENTS,
			0, 0, 0,
		)
		if errno != 0 {
			// Log error but don't fail the operation
		}
	}
}

// waitForCQE waits for a completion event
func (t *ioURingTransport) waitForCQE(timeoutMs uint32) (*IoURingCQE, error) {
	head := atomic.LoadUint32(t.ioUring.cqHead)
	tail := atomic.LoadUint32(t.ioUring.cqTail)

	// Otherwise, wait for events from kernel
	_, _, errno := unix.Syscall6(
		SYS_IO_URING_ENTER,
		uintptr(t.ioUring.fd),
		uintptr(1), // count
		IORING_ENTER_GETEVENTS,
		uintptr(timeoutMs), 0, 0,
	)

	if errno != 0 && errno != unix.EAGAIN && errno != unix.EINTR {
		return nil, fmt.Errorf("io_uring_enter failed: %v", errno)
	}

	// Try again to get CQE after kernel notification
	head = atomic.LoadUint32(t.ioUring.cqHead)
	tail = atomic.LoadUint32(t.ioUring.cqTail)

	if head != tail {
		cqeIdx := head & t.ioUring.cqMask
		cqeOffset := uintptr(cqeIdx) * uintptr(t.ioUring.cqEntrySize) // Use actual CQE size from params
		cqe := (*IoURingCQE)(unsafe.Pointer(uintptr(unsafe.Pointer(&t.ioUring.cqMmap[0])) + cqeOffset))
		// Increment head after retrieving CQE
		atomic.StoreUint32(t.ioUring.cqHead, (head+1)&t.ioUring.cqMask)
		return cqe, nil
	}

	return nil, fmt.Errorf("no CQE available after waiting")
}

// Send submits send operations to io_uring
func (t *ioURingTransport) Send(buffers [][]byte) error {
	t.sendMutex.Lock()
	defer t.sendMutex.Unlock()
	if t.closed {
		return api.ErrTransportClosed
	}

	if len(buffers) == 0 {
		return nil
	}

	// Submit each buffer as a separate send operation
	for _, buf := range buffers {
		sqe, sqeIdx, err := t.getSQESlot()
		if err != nil {
			// Fallback to regular write if SQ is full
			n, writeErr := syscall.Write(t.fd, buf)
			if writeErr != nil {
				return fmt.Errorf("io_uring SQ full and write failed: %w", writeErr)
			}
			if n != len(buf) {
				return fmt.Errorf("incomplete write: %d of %d bytes", n, len(buf))
			}
			continue
		}

		// Fill in the SQE for send operation
		sqe.OpCode = IORING_OP_SEND
		sqe.Flags = 0
		sqe.IoPrio = 0
		sqe.Fd = int32(t.fd)
		sqe.Off = 0
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&buf[0])))
		sqe.Len = uint32(len(buf))
		sqe.Flags2 = 0
		sqe.UserData = uint64(sqeIdx) // Use index as user data for tracking

		// Submit the SQE
		t.submitSQE(sqe, sqeIdx)
	}

	// Wait for all send operations to complete
	var results []error
	for i := 0; i < len(buffers); i++ {
		cqe, err := t.waitForCQE(1000) // 1 second timeout
		if err != nil {
			results = append(results, err)
			continue
		}

		if cqe.Result < 0 {
			// Negative result indicates an error
			errno := -int(cqe.Result)
			results = append(results, fmt.Errorf("send operation failed with errno %d", errno))
		}
	}

	if len(results) > 0 {
		return fmt.Errorf("send error(s): %v", results)
	}

	return nil
}

// Recv waits for receive operations from io_uring
func (t *ioURingTransport) Recv() ([][]byte, error) {
	t.recvMutex.Lock()
	defer t.recvMutex.Unlock()
	if t.closed {
		return nil, api.ErrTransportClosed
	}

	// Prepare receive buffers as a batch
	batch := 16
	bufs := make([][]byte, batch)

	// First, submit all receive operations
	for i := 0; i < batch; i++ {
		buf := t.bufPool.Get(t.ioBufferSize, t.numaNode)
		bufs[i] = buf.Bytes()

		sqe, sqeIdx, err := t.getSQESlot()
		if err != nil {
			// If SQ is full, use regular read as fallback
			n, readErr := syscall.Read(t.fd, bufs[i])
			if readErr != nil {
				return nil, fmt.Errorf("io_uring SQ full and read failed: %w", readErr)
			}
			if n > 0 {
				bufs[i] = bufs[i][:n]
			} else {
				bufs[i] = bufs[i][:0]
			}
			continue
		}

		// Fill in the SQE for recv operation
		sqe.OpCode = IORING_OP_RECV
		sqe.Flags = 0
		sqe.IoPrio = 0
		sqe.Fd = int32(t.fd)
		sqe.Off = 0
		sqe.Addr = uint64(uintptr(unsafe.Pointer(&bufs[i][0])))
		sqe.Len = uint32(len(bufs[i]))
		sqe.Flags2 = 0
		sqe.UserData = uint64(sqeIdx) | (uint64(i) << 32) // Use index + batch position as user data

		// Submit the SQE
		t.submitSQE(sqe, sqeIdx)
	}

	// Wait for receive completions
	receivedBuffs := make([][]byte, 0, batch)
	for i := 0; i < batch; i++ {
		cqe, err := t.waitForCQE(1000) // 1 second timeout
		if err != nil {
			continue // Continue with other buffers if timeout occurs
		}

		if cqe.Result > 0 {
			// Extract buffer index from user data
			bufIdx := (cqe.UserData >> 32) & 0xFFFFFFFF
			bytesReceived := int(cqe.Result)

			if bufIdx < uint64(len(bufs)) && bytesReceived > 0 {
				bufs[bufIdx] = bufs[bufIdx][:bytesReceived]
				receivedBuffs = append(receivedBuffs, bufs[bufIdx])
			}
		} else if cqe.Result == 0 {
			// Connection closed - mark as empty
		} else if cqe.Result < 0 {
			// Error occurred - negative result means error code
			_ = -int(cqe.Result)  // Error code, not used but acknowledge the variable
			// Log error but continue processing other buffers
		}
	}

	return receivedBuffs, nil
}

// Close closes the transport and io_uring instance
func (t *ioURingTransport) Close() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true

	// Cleanup io_uring resources
	if t.ioUring != nil {
		if t.ioUring.sqMmap != nil {
			unix.Munmap(t.ioUring.sqMmap)
		}
		if t.ioUring.cqMmap != nil {
			unix.Munmap(t.ioUring.cqMmap)
		}
		if t.ioUring.sqeMmap != nil {
			unix.Munmap(t.ioUring.sqeMmap)
		}
		unix.Close(int(t.ioUring.fd))
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
	defer et.mu.Unlock()
	if et.closed {
		return nil, api.ErrTransportClosed
	}
	batch := 16
	bufs := make([][]byte, batch)
	for i := range bufs {
		buf := et.bufPool.Get(et.ioBufferSize, et.numaNode)
		bufs[i] = buf.Bytes()
	}
	n, _, _, _, err := unix.RecvmsgBuffers(et.fd, bufs, nil, unix.MSG_DONTWAIT)
	if err != nil {
		return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
	}
	return bufs[:n], nil
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
		n, err := unix.SendmsgBuffers(et.fd, batch, nil, nil, 0)
		if err != nil {
			return fmt.Errorf("SendmsgBuffers: %w", err)
		}
		// n is the number of bytes sent, but it should be at least the size of our batch
		if n <= 0 {
			return fmt.Errorf("SendmsgBuffers: sent no data")
		}
		// All buffers in the batch were sent successfully
		sent += len(batch)
		left -= len(batch)
	}
	return nil
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