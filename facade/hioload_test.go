package facade_test

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/facade"
)

// Test the full lifecycle, including registration and removal of handlers,
// context factory, debug API, and hot-reload hook registration.
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
	// Register and unregister a test handler
	var handler testHandler
	if err := h.RegisterHandler(&handler); err != nil {
		t.Fatal(err)
	}
	if err := h.UnregisterHandler(&handler); err != nil {
		t.Fatal(err)
	}
	// Test context factory retrieval
	cf := h.GetContextFactory()
	ctx := cf.NewContext()
	if ctx == nil {
		t.Error("ContextFactory did not return context")
	}
	// Test debug API retrieval
	dbg := h.GetDebugAPI()
	if dbg == nil {
		t.Error("Debug API not returned")
	}
	// Test register reload hook
	called := false
	h.RegisterReloadHook(func() { called = true })
	h.GetControl().SetConfig(map[string]any{"some": "data"})
	time.Sleep(10 * time.Millisecond)
	if !called {
		t.Error("Reload hook not triggered")
	}
	if err := h.Shutdown(); err != nil {
		t.Error(err)
	}
}

type testHandler struct{}

func (t *testHandler) Handle(data any) error { return nil }
