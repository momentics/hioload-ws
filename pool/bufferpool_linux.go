//go:build linux
// +build linux

package pool

import (
	"sync"
	"syscall"

	"github.com/momentics/hioload-ws/api"
)

type linuxBuffer struct {
	data   []byte
	pool   *linuxBufferPool
	numaID int
}

func (b *linuxBuffer) Bytes() []byte { return b.data }
func (b *linuxBuffer) Release()      { b.pool.recycle(b) }
func (b *linuxBuffer) Copy() []byte  { c := make([]byte, len(b.data)); copy(c, b.data); return c }
func (b *linuxBuffer) NUMANode() int { return b.numaID }
func (b *linuxBuffer) Slice(from, to int) api.Buffer {
	return &linuxBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID}
}

type linuxBufferPool struct {
	pools map[int]chan *linuxBuffer
	mu    sync.Mutex
}

func newBufferPool(numaNode int) api.BufferPool {
	return &linuxBufferPool{pools: map[int]chan *linuxBuffer{numaNode: make(chan *linuxBuffer, 1024)}}
}

func (p *linuxBufferPool) Get(size, numaPref int) api.Buffer {
	p.mu.Lock()
	ch, ok := p.pools[numaPref]
	if !ok {
		ch = make(chan *linuxBuffer, 1024)
		p.pools[numaPref] = ch
	}
	p.mu.Unlock()
	select {
	case buf := <-ch:
		if cap(buf.data) < size {
			buf.data = make([]byte, size)
		} else {
			buf.data = buf.data[:size]
		}
		return buf
	default:
		length := ((size + (2 << 20) - 1) / (2 << 20)) * (2 << 20)
		data, err := syscall.Mmap(-1, 0, length, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE|syscall.MAP_HUGETLB)
		if err != nil {
			return &linuxBuffer{data: make([]byte, size), pool: p, numaID: numaPref}
		}
		return &linuxBuffer{data: data[:size], pool: p, numaID: numaPref}
	}
}

func (p *linuxBufferPool) Put(b api.Buffer) {
	if lb, ok := b.(*linuxBuffer); ok {
		select {
		case p.pools[lb.numaID] <- lb:
		default:
		}
	}
}

func (p *linuxBufferPool) Stats() api.BufferPoolStats { return api.BufferPoolStats{} }

func (p *linuxBufferPool) recycle(b *linuxBuffer) { p.Put(b) }
