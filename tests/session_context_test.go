// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// session_context_test.go — Tests for custom Context store: propagation, TTL, clone.
package tests

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/internal/session"
)

func TestContext_PropagationAndTTL(t *testing.T) {
	ctx := session.NewSessionManager(1).Create("sess1")
	store := ctx.Context()

	store.Set("keyA", 123, true)
	store.Set("keyB", 42, false)

	if v, ok := store.Get("keyA"); !ok || v.(int) != 123 {
		t.Error("Failed to get propagated keyA")
	}
	if !store.IsPropagated("keyA") {
		t.Error("keyA should be propagated")
	}
	if store.IsPropagated("keyB") {
		t.Error("keyB should not be propagated")
	}

	store.WithExpiration("keyA", 10_000_000) // 10ms
	time.Sleep(20 * time.Millisecond)
	if _, ok := store.Get("keyA"); ok {
		t.Error("keyA should be expired")
	}

	clone := store.Clone()
	if v, ok := clone.Get("keyB"); !ok || v.(int) != 42 {
		t.Error("Clone failed to copy keyB")
	}
}
