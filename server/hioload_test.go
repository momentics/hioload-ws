// File: facade/hioload_test.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Test the full HioloadWS lifecycle, including explicit Shutdown() method.

package server_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/server"
)

type testHandler struct{}

func (h *testHandler) Handle(data any) error { return nil }

func TestHioloadWSFullLifecycle(t *testing.T) {
	// Create facade with default config
	h, err := server.New(server.DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Start the facade subsystems
	if err := h.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Test task submission
	var executed atomic.Bool
	if err := h.Submit(func() { executed.Store(true) }); err != nil {
		t.Fatalf("Submit() error: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if !executed.Load() {
		t.Error("Submitted task did not execute")
	}

	// Register and unregister a handler
	handler := &testHandler{}
	if err := h.RegisterHandler(handler); err != nil {
		t.Fatalf("RegisterHandler() error: %v", err)
	}
	if err := h.UnregisterHandler(handler); err != nil {
		t.Fatalf("UnregisterHandler() error: %v", err)
	}

	// Test context factory
	ctx := h.GetContextFactory().NewContext()
	if ctx == nil {
		t.Error("ContextFactory.NewContext() returned nil")
	}

	// Test debug API
	if dbg := h.GetDebugAPI(); dbg == nil {
		t.Error("GetDebugAPI() returned nil")
	}

	// Test Shutdown (formerly Stop) method
	if err := h.Shutdown(); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
	// Second call to Shutdown should be idempotent
	if err := h.Shutdown(); err != nil {
		t.Errorf("Second Shutdown() error: %v", err)
	}
}
