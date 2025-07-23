package pool_test

import (
	"testing"

	"github.com/momentics/hioload-ws/pool"
)

func TestBufferPoolReuse(t *testing.T) {
	mgr := pool.NewBufferPoolManager()
	bp := mgr.GetPool(-1)
	b1 := bp.Get(128, -1)
	b1.Release()
	b2 := bp.Get(64, -1)
	// b2 should reuse underlying storage
	if cap(b2.Bytes()) < 128 {
		t.Error("Buffer capacity too small; reuse failed")
	}
}
