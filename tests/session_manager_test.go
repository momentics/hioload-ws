// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// session_manager_test.go — Unit and concurrency tests for SessionManager.
package tests

import (
	"sync"
	"testing"
	"github.com/momentics/hioload-ws/internal/session"
)

// TestNewSessionManager_SingleThreaded checks correctness of CRUD operations.
func TestNewSessionManager_SingleThreaded(t *testing.T) {
	sm := session.NewSessionManager(8)
	id := "client-123"
	sess, err := sm.Create(id)
	if err != nil {
		t.Fatal("Failed to create session:", err)
	}
	s1, ok := sm.Get(id)
	if !ok || s1.ID() != id {
		t.Fatal("Get did not return correct session")
	}
	sm.Delete(id)
	_, ok = sm.Get(id)
	if ok {
		t.Error("Expected session deleted")
	}
}

// TestSessionManager_ConcurrentAccess validates thread-safe operation.
func TestSessionManager_ConcurrentAccess(t *testing.T) {
	sm := session.NewSessionManager(32)
	var wg sync.WaitGroup
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = "sess-" + string(i)
	}
	// Concurrent creation
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sm.Create(id)
		}(id)
	}
	wg.Wait()

	// Concurrent retrieval
	wg = sync.WaitGroup{}
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, ok := sm.Get(id); !ok {
				t.Errorf("Session missing: %s", id)
			}
		}(id)
	}
	wg.Wait()
}
