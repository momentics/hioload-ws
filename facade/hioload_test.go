package facade_test

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/facade"
)

func TestHioloadWSFullLifecycle(t *testing.T) {
	h, err := facade.New(facade.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Start(); err != nil {
		t.Fatal(err)
	}
	executed := false
	if err := h.Submit(func() { executed = true }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if !executed {
		t.Error("Executor failed to run task")
	}
	if err := h.Shutdown(); err != nil {
		t.Error(err)
	}
}
