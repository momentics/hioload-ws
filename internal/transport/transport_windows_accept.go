// File: internal/transport/transport_windows_accept.go
//go:build windows
// +build windows

//
// Windows-specific native AcceptEx and TransmitPackets zero-copy implementation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package transport

import (
	"fmt"
	"net"
	"os"
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
	"golang.org/x/sys/windows"
)

// TransmitPackets API constants
const (
	TP_ELEMENT_MEMORY = 0x00000001
	TP_DISCONNECT     = 0x00000001
	TP_REUSE_SOCKET   = 0x00000002
	TP_USE_KERNEL_APC = 0x00000000 // use kernel APC (0)
)

// TRANSMIT_PACKETS_ELEMENT flags:
// dwElFlags == TP_ELEMENT_MEMORY -> Buffer pointer used
type TRANSMIT_PACKETS_ELEMENT struct {
	dwElFlags uint32
	cLength   uint32
	reserved  uint32
	pBuffer   uintptr // pointer to data when TP_ELEMENT_MEMORY
}

var (
	modmswsock          = windows.NewLazySystemDLL("Mswsock.dll")
	procAcceptEx        = modmswsock.NewProc("AcceptEx")
	procTransmitPackets = modmswsock.NewProc("TransmitPackets")
)

// ListenerEx wraps net.Listener to perform native zero-copy AcceptEx and TransmitPackets.
type ListenerEx struct {
	ln          net.Listener
	acceptSock  windows.Handle
	bufPool     api.BufferPool
	iocp        windows.Handle
	channelSize int
}

// NewListenerEx creates a new ListenerEx on the given address.
func NewListenerEx(addr string, bufPool api.BufferPool, channelSize int) (*ListenerEx, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	tcpLn := ln.(*net.TCPListener)
	file, err := tcpLn.File()
	if err != nil {
		ln.Close()
		return nil, err
	}
	acceptSock := windows.Handle(file.Fd())
	iocp, err := windows.CreateIoCompletionPort(acceptSock, 0, 0, 0)
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("CreateIoCompletionPort: %w", err)
	}
	return &ListenerEx{
		ln:          ln,
		acceptSock:  acceptSock,
		bufPool:     bufPool,
		iocp:        iocp,
		channelSize: channelSize,
	}, nil
}

// Accept uses AcceptEx for asynchronous zero-copy accept.
func (l *ListenerEx) Accept() (*protocol.WSConnection, error) {
	clientSock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, err
	}
	overl := new(windows.Overlapped)
	r1, _, e1 := procAcceptEx.Call(
		uintptr(l.acceptSock),
		uintptr(clientSock),
		0,
		0,
		uintptr(unsafe.Sizeof(windows.SockaddrInet4{})),
		uintptr(unsafe.Sizeof(windows.SockaddrInet4{})),
		0,
		uintptr(unsafe.Pointer(overl)),
	)
	if r1 == 0 && e1 != windows.ERROR_IO_PENDING {
		windows.Closesocket(clientSock)
		return nil, fmt.Errorf("AcceptEx failed: %v", e1)
	}
	var n uint32
	err = windows.GetQueuedCompletionStatus(l.iocp, &n, nil, (**windows.Overlapped)(unsafe.Pointer(&overl)), windows.INFINITE)
	if err != nil {
		windows.Closesocket(clientSock)
		return nil, fmt.Errorf("GetQueuedCompletionStatus accept: %w", err)
	}
	_ = windows.SetsockoptInt(clientSock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)
	file := os.NewFile(uintptr(clientSock), "")
	connNet, err := net.FileConn(file)
	if err != nil {
		windows.Closesocket(clientSock)
		return nil, err
	}
	tr := &connTransport{conn: connNet, bufferPool: l.bufPool}
	return protocol.NewWSConnection(tr, l.bufPool, l.channelSize), nil
}

// Transmit sends buffers using TransmitPackets for zero-copy.
func (l *ListenerEx) Transmit(conn windows.Handle, bufs [][]byte) error {
	pktCount := uint32(len(bufs))
	elements := make([]TRANSMIT_PACKETS_ELEMENT, pktCount)
	for i, b := range bufs {
		elements[i].dwElFlags = TP_ELEMENT_MEMORY
		elements[i].cLength = uint32(len(b))
		elements[i].pBuffer = uintptr(unsafe.Pointer(&b[0]))
	}
	overl := new(windows.Overlapped)
	r1, _, err := procTransmitPackets.Call(
		uintptr(conn),
		uintptr(unsafe.Pointer(&elements[0])),
		uintptr(pktCount),
		0,
		uintptr(unsafe.Pointer(overl)),
		uintptr(TP_USE_KERNEL_APC),
	)
	if r1 == 0 {
		return fmt.Errorf("TransmitPackets failed: %v", err)
	}
	err = windows.GetQueuedCompletionStatus(l.iocp, nil, nil, (**windows.Overlapped)(unsafe.Pointer(&overl)), windows.INFINITE)
	if err != nil {
		return fmt.Errorf("GetQueuedCompletionStatus transmit: %w", err)
	}
	return nil
}

// Close releases the IOCP handle and closes the listener.
func (l *ListenerEx) Close() error {
	_ = windows.CloseHandle(l.iocp)
	return l.ln.Close()
}
