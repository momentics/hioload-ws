// File: pool/base_bufferpool.go
// Package pool
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Общая реализация BufferPool без учёта платформы.

package pool

import (
    "sync"

    "github.com/momentics/hioload-ws/api"
)

type bufferFactory[T api.Buffer] func(size int, numaPref int) T

type baseBufferPool[T api.Buffer] struct {
    pools   map[int]chan T
    mu      sync.Mutex
    factory bufferFactory[T]
}

func newBaseBufferPool[T api.Buffer](numaNode int, factory bufferFactory[T]) *baseBufferPool[T] {
    return &baseBufferPool[T]{
        pools:   map[int]chan T{numaNode: make(chan T, 1024)},
        factory: factory,
    }
}

func (p *baseBufferPool[T]) getChannel(numaPref int) chan T {
    p.mu.Lock()
    ch, ok := p.pools[numaPref]
    if !ok {
        ch = make(chan T, 1024)
        p.pools[numaPref] = ch
    }
    p.mu.Unlock()
    return ch
}

func (p *baseBufferPool[T]) Get(size, numaPref int) api.Buffer {
    ch := p.getChannel(numaPref)
    select {
    case buf := <-ch:
        if cap(buf.Bytes()) < size {
            return p.factory(size, numaPref)
        }
        return buf.Slice(0, size)
    default:
        return p.factory(size, numaPref)
    }
}

func (p *baseBufferPool[T]) Put(b api.Buffer) {
    if tb, ok := b.(T); ok {
        ch := p.getChannel(tb.NUMANode())
        select {
        case ch <- tb:
        default:
        }
    }
}

func (p *baseBufferPool[T]) Stats() api.BufferPoolStats {
    // Заглушка: статистика может быть расширена
    return api.BufferPoolStats{}
}

func (p *baseBufferPool[T]) recycle(b T) {
    p.Put(b)
}
