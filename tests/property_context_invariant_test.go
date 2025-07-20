// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// property_context_invariant_test.go — Property-based test for context invariants.
package tests

import (
	"testing"

	"github.com/momentics/hioload-ws/internal/session"
)

func TestContextPropagation_Property(t *testing.T) {
	ctx := session.NewSessionManager(4)
	for i := 0; i < 100; i++ {
		sess, _ := ctx.Create("s" + string(i))
		store := sess.Context()
		store.Set("k", i, i%2 == 0)
		v, ok := store.Get("k")
		if !ok || v.(int) != i {
			t.Errorf("Key propagation failed for i=%d, got %v", i, v)
		}
		if i%2 == 0 && !store.IsPropagated("k") {
			t.Errorf("Propagation not tracked for i=%d", i)
		}
	}
}
