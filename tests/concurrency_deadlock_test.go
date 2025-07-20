// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// concurrency_deadlock_test.go — Simulates heavy concurrent access to SessionManager.
package tests

import (
	"sync"
	"testing"
	"time"
	"github.com/momentics/hioload-ws/internal/session"
)

func TestSessionManager_HeavyConcurrency(t *testing.T) {
	sm := session.NewSessionManager(32)
	const N = 500
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Massive parallel create/delete/get
	for i := 0; i < N; i++ {
		id := "concurrent-" + string(rune(i))
		wg.Add(3)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				_, _ = sm.Create(id)
			}
		}(id)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				sm.Delete(id)
			}
		}(id)
		go func(id string) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_, _ = sm.Get(id)
			}
		}(id)
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout: possible deadlock or excessive contention")
	}
}
