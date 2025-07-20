// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// poller_adapter_integration_test.go — Integration test for adapters.PollerAdapter.
package tests

import (
	"sync"
	"testing"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
)

type testHandler struct {
	mu    sync.Mutex
	count int
}

func (th *testHandler) Handle(data any) error {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.count++
	return nil
}

func TestPollerAdapter_Integration(t *testing.T) {
	poller := adapters.NewPollerAdapter(8, 128)
	handler := &testHandler{}

	if err := poller.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Simulate events via internal event loop (normally done via transport)
	for i := 0; i < 20; i++ {
		if handled, err := poller.Poll(1); err != nil || handled < 0 {
			t.Fatal("Poll failed or handled < 0")
		}
	}

	poller.Stop()
}
