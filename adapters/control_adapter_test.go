package adapters_test

import (
	"testing"

	"github.com/momentics/hioload-ws/adapters"
)

func TestControlAdapterBasic(t *testing.T) {
	ctrl := adapters.NewControlAdapter()
	cfg := ctrl.GetConfig()
	if len(cfg) != 0 {
		t.Error("Expected empty config on init")
	}
	err := ctrl.SetConfig(map[string]any{"k": 1})
	if err != nil {
		t.Fatal(err)
	}
	stats := ctrl.Stats()
	if stats["k"] != 1 {
		t.Error("SetConfig did not apply")
	}
	called := false
	ctrl.OnReload(func() { called = true })
	ctrl.SetConfig(map[string]any{"x": 2})
	// allow hook
	if !called {
		t.Error("Reload hook not called")
	}
}
