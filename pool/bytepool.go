// File: pool/bytepool.go
// Author: momentics <momentics@gmail.com>

package pool


// BytePool is compatible with NUMA-pool if enabled.
type BytePool struct {
	npool *NUMAPool // If set, use NUMA-aware pool, fallback to sync.Pool.
	size  int
}

func NewBytePool(size int, node int, useNUMA bool) *BytePool {
	return &BytePool{
		npool: NewNUMAPool(node, size, useNUMA),
		size:  size,
	}
}

// GetBuffer returns a buffer from the pool.
func (b *BytePool) GetBuffer() []byte {
	if b.npool != nil && b.npool.enable {
		return b.npool.Get()
	}
	// fallback: make regular slice
	return make([]byte, b.size)
}

// PutBuffer returns a buffer to the pool.
func (b *BytePool) PutBuffer(buf []byte) {
	if b.npool != nil && b.npool.enable {
		b.npool.Put(buf)
		return
	}
	// fallback: GC handles memory
}
