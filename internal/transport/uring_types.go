//go:build linux
// +build linux

// File: internal/transport/uring_types.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Shared io_uring types and constants for Linux transport implementations.

package transport

const (
	// io_uring opcodes
	IORING_SETUP_SQPOLL = 1 << 6
	IORING_SETUP_IOPOLL = 1 << 2
	IORING_SETUP_SQ_AFF = 1 << 3
	IORING_SETUP_CQSIZE = 1 << 2
	IORING_SETUP_CLAMP  = 1 << 4

	IORING_OP_NOP = 0
	IORING_OP_READV = 1
	IORING_OP_WRITEV = 2
	IORING_OP_FSYNC = 3
	IORING_OP_READ_FIXED = 4
	IORING_OP_WRITE_FIXED = 5
	IORING_OP_POLL_ADD = 6
	IORING_OP_POLL_REMOVE = 7
	IORING_OP_CONNECT = 8
	IORING_OP_ACCEPT = 9
	IORING_OP_FALLOCATE = 10
	IORING_OP_OPENAT = 11
	IORING_OP_CLOSE = 12
	IORING_OP_FILES_UPDATE = 13
	IORING_OP_STATX = 14
	IORING_OP_READ = 15
	IORING_OP_WRITE = 16
	IORING_OP_FADVISE = 17
	IORING_OP_MADVISE = 18
	IORING_OP_SEND = 26
	IORING_OP_RECV = 27
	IORING_OP_OPENAT2 = 21
	IORING_OP_EPOLL_CTL = 22
	IORING_OP_SPLICE = 23
	IORING_OP_PROVIDE_BUFFERS = 24
	IORING_OP_REMOVE_BUFFERS = 25
	IORING_OP_TEE = 26
	IORING_OP_TIMEOUT = 27
	IORING_OP_TIMEOUT_REMOVE = 28
	IORING_OP_ACCEPT_DIRECT = 29
	IORING_OP_POLL_ADD_MULTI = 30
	IORING_OP_WAIT_WHILE = 31
	IORING_OP_SEND_ZC = 32
	IORING_OP_SENDMSG_ZC = 33
	IORING_OP_RECVMSG = 34

	SYS_IO_URING_SETUP = 425
	SYS_IO_URING_ENTER = 426
	SYS_IO_URING_REGISTER = 427

	// io_uring flags
	IORING_SQ_NEED_WAKEUP = 1 << 0  // 1

	IORING_ENTER_GETEVENTS = 1
	IORING_ENTER_SQ_WAKEUP = 2
	IORING_ENTER_SQ_WAIT = 4
	IORING_ENTER_EXT_ARG = 8
)

// IoURingParams represents parameters for io_uring setup
type IoURingParams struct {
	SQEntries    uint32
	CQEntries    uint32
	Flags        uint32
	SQEntrySize  uint32
	CQEntrySize  uint32
	WorkerNr     uint32
	CQOffEventfd uint32
	CQOffUserData uint32
	CQOffFlags   uint32
	SQOffHead    uint32
	SQOffTail    uint32
	SQOffRingMask uint32
	SQOffRingEntries uint32
	SQOffFlags   uint32
	SQOffArray   uint32
	CQOffHead    uint32
	CQOffTail    uint32
	CQOffRingMask uint32
	CQOffRingEntries uint32
	CQOffOverflow uint32
	CQOffCqes    uint32
}

// IoURingSQE represents a submission queue entry
type IoURingSQE struct {
	OpCode    uint8
	Flags     uint8
	IoPrio    uint16
	Fd        int32
	Off       uint64
	Addr      uint64
	Len       uint32
	Flags2    uint32
	UserData  uint64
	Pad       [2]uint64
}

// IoURingCQE represents a completion queue entry
type IoURingCQE struct {
	UserData    uint64
	Result      int32
	Flags       uint32
	ExtraData   [4]uint64 // For extended CQE data if needed
}

// IoURing represents the io_uring instance
type IoURing struct {
	fd            int32
	sqHead        *uint32
	sqTail        *uint32
	sqMask        uint32
	sqFlags       *uint32
	cqHead        *uint32
	cqTail        *uint32
	cqMask        uint32
	cqOverflow    *uint32

	sqPtrs        []uintptr // Submission queue entries pointers
	cqPtrs        []uintptr // Completion queue entries pointers

	sqMmap        []byte
	cqMmap        []byte
	sqeMmap       []byte   // Submission queue entries mmap

	sqSize        uint64
	cqSize        uint64
	sqeSize       uint64

	sqeHead       uint32
	sqeTail       uint32
	sqeMask       uint32

	// Offsets for accessing ring buffer elements
	sqOffArray    uint32
	sqEntrySize   uint32
	cqEntrySize   uint32
}